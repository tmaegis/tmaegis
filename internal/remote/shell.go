// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Timings of the interactive shell; variables so tests can shorten them.
var (
	shellQuiet       = 250 * time.Millisecond // no output for this long = the device is waiting
	shellPromptNudge = 2 * time.Second        // send a newline if no prompt appeared by then
	shellLoosePrompt = 2 * time.Second        // accept a changed prompt after this much silence
)

var (
	// promptRe matches CLI prompts: "R1#", "user@mx> ", "FGT60E # ", "RP/0/RSP0/CPU0:xr#",
	// "root@(bigip)(cfg-sync Standalone)(Active)(/Common)(tmos)# ".
	promptRe = regexp.MustCompile(`^[A-Za-z0-9_.\-@:/~()\[\]{} ]{1,160}[>#$%]\s?$`)
	// pagerRe matches "--More--", " -- More -- ", "---(more)---", "---(more 42%)---",
	// "<--- More --->", "Press any key to continue", "lines 1-40".
	pagerRe = regexp.MustCompile(`(?i)(?:-+ ?\(?more[^-\n]*\)? ?-+>?|<-+ more -+>|press any key to continue[^\n]*|\(q\)uit[^\n]*|lines \d+-\d+(?:/\d+)?(?: \(end\))?)\s*$`)
	// pagerAnywhere strips pager residue the device did not erase itself.
	pagerAnywhere = regexp.MustCompile(`(?i) ?(?:--More--|-- More --|---\(more[^)\n]*\)---|<--- More --->) ?`)
	ansiRe        = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[ -/]*[@-~]|[()][A-Za-z0-9]|[=>78DEHMc])`)
)

// Shell is an interactive CLI session on a network device. Only commands that pass
// CheckCommand can be sent.
type Shell struct {
	c     *Client
	sess  *ssh.Session
	stdin io.WriteCloser

	mu     sync.Mutex
	buf    []byte
	eof    bool
	notify chan struct{}

	prompt  string
	closeMu sync.Once
}

// OpenShell starts a pty shell and waits for the first prompt.
func (c *Client) OpenShell(ctx context.Context) (*Shell, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	sess, err := c.ssh.NewSession()
	if err != nil {
		c.release()
		return nil, fmt.Errorf("open session: %w", err)
	}
	s := &Shell{c: c, sess: sess, notify: make(chan struct{}, 1)}
	fail := func(err error) (*Shell, error) { s.Close(); return nil, err }
	// A wide terminal avoids line wrapping inside long configuration lines.
	if err := sess.RequestPty("vt100", 0, 511, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400}); err != nil {
		return fail(fmt.Errorf("request pty: %w", err))
	}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		return fail(err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	if err := sess.Shell(); err != nil {
		return fail(fmt.Errorf("start shell: %w", err))
	}
	go s.read(stdout)

	ctx, cancel := context.WithTimeout(ctx, c.cfg.CommandTimeout)
	defer cancel()
	nudged := false
	started := time.Now()
	clean, err := s.waitFor(ctx, 0, func(clean string, idle bool) bool {
		if idle && promptRe.MatchString(lastLine(clean)) {
			return true
		}
		if idle && !nudged && time.Since(started) > shellPromptNudge {
			nudged = true
			_, _ = s.stdin.Write([]byte("\n"))
		}
		return false
	})
	if err != nil {
		return fail(fmt.Errorf("waiting for the device prompt: %w", err))
	}
	s.prompt = strings.TrimSpace(lastLine(clean))
	return s, nil
}

func (s *Shell) read(r io.Reader) {
	b := make([]byte, 32<<10)
	for {
		n, err := r.Read(b)
		s.mu.Lock()
		s.buf = append(s.buf, b[:n]...)
		if err != nil {
			s.eof = true
		}
		s.mu.Unlock()
		select {
		case s.notify <- struct{}{}:
		default:
		}
		if err != nil {
			return
		}
	}
}

// Prompt is the detected device prompt.
func (s *Shell) Prompt() string { return s.prompt }

// Run sends one allow-listed command and returns its output without the echo and prompt.
func (s *Shell) Run(ctx context.Context, cmd string) ([]byte, error) {
	if err := CheckCommand(cmd); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.c.cfg.CommandTimeout)
	defer cancel()
	s.mu.Lock()
	start := len(s.buf)
	s.mu.Unlock()
	if _, err := s.stdin.Write([]byte(cmd + "\n")); err != nil {
		return nil, err
	}
	var quietSince time.Time
	clean, err := s.waitFor(ctx, start, func(clean string, idle bool) bool {
		ll := strings.TrimSpace(lastLine(clean))
		if !idle {
			quietSince = time.Time{}
			return false
		}
		if quietSince.IsZero() {
			quietSince = time.Now()
		}
		// The command is done when the prompt is back and the device is quiet.
		return ll == s.prompt && strings.Contains(clean, "\n") ||
			promptRe.MatchString(ll) && time.Since(quietSince) >= shellLoosePrompt
	})
	if err != nil {
		return nil, err
	}
	return []byte(trimEchoAndPrompt(clean, cmd)), nil
}

// waitFor collects output from offset start until done says so, answering pagers on the way.
func (s *Shell) waitFor(ctx context.Context, start int, done func(clean string, idle bool) bool) (string, error) {
	pagerAt := start
	lastLen := -1
	lastChange := time.Now()
	for {
		s.mu.Lock()
		raw := append([]byte(nil), s.buf[start:]...)
		eof := s.eof
		s.mu.Unlock()
		if len(raw) > s.c.cfg.MaxOutput {
			return "", fmt.Errorf("%w (%d bytes)", ErrOutputTooLarge, s.c.cfg.MaxOutput)
		}
		if len(raw) != lastLen {
			lastLen, lastChange = len(raw), time.Now()
		}
		idle := time.Since(lastChange) >= shellQuiet
		if tail := terminalText(string(raw[pagerAt-start:]), false); pagerRe.MatchString(lastLine(tail)) && !promptRe.MatchString(lastLine(tail)) {
			if _, err := s.stdin.Write([]byte(" ")); err != nil {
				return "", err
			}
			pagerAt = start + len(raw)
			continue
		}
		clean := cleanTerminal(string(raw))
		if done(clean, idle) {
			return clean, nil
		}
		if eof {
			return clean, errors.New("the device closed the session")
		}
		select {
		case <-s.notify:
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return clean, fmt.Errorf("device %w after %s", ErrTimeout, s.c.cfg.CommandTimeout)
			}
			return clean, ctx.Err()
		case <-time.After(shellQuiet / 2):
		}
	}
}

// Close leaves the CLI and frees the session slot.
func (s *Shell) Close() error {
	s.closeMu.Do(func() {
		if s.stdin != nil {
			_, _ = s.stdin.Write([]byte("exit\n"))
		}
		s.sess.Close()
		s.c.release()
	})
	return nil
}

// cleanTerminal removes ANSI escapes, applies backspaces and bare carriage returns, and
// strips pager residue, so the text reads like a saved configuration file.
func cleanTerminal(s string) string { return terminalText(s, true) }

func terminalText(s string, stripPager bool) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		var cur []rune
		for _, r := range line {
			switch r {
			case '\b', 0x7f:
				if len(cur) > 0 {
					cur = cur[:len(cur)-1]
				}
			case '\r':
				cur = cur[:0]
			case 0x07, 0x00:
			default:
				cur = append(cur, r)
			}
		}
		l := string(cur)
		if !stripPager {
			out = append(out, l)
			continue
		}
		l = pagerAnywhere.ReplaceAllString(l, "")
		if strings.TrimSpace(l) == "" && strings.TrimSpace(string(cur)) != "" {
			continue // a line that held only a pager prompt
		}
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return strings.Join(out, "\n")
}

func lastLine(s string) string {
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// trimEchoAndPrompt drops the echoed command line and the trailing prompt line.
func trimEchoAndPrompt(clean, cmd string) string {
	lines := strings.Split(clean, "\n")
	if len(lines) > 0 && strings.HasSuffix(strings.TrimSpace(lines[0]), strings.TrimSpace(cmd)) {
		lines = lines[1:]
	}
	if len(lines) > 0 && promptRe.MatchString(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	out := strings.Join(lines, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}
