// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf16"

	"golang.org/x/crypto/ssh"
)

// capWriter keeps at most limit bytes and records an overflow.
type capWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	limit    int
	overflow bool
	onFull   func()
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.overflow {
		return len(p), nil
	}
	if room := w.limit - w.buf.Len(); len(p) > room {
		w.buf.Write(p[:max(room, 0)])
		w.overflow = true
		if w.onFull != nil {
			go w.onFull()
		}
		return len(p), nil
	}
	return w.buf.Write(p)
}

func (w *capWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf.Bytes()...)
}

// acquire takes a session slot, honouring ctx.
func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) release() { <-c.sem }

// Exec runs one command on an exec channel and returns its combined output. It is bounded by
// CommandTimeout and MaxOutput; a non-zero exit returns the output with an *ssh.ExitError.
// Exec does not check the command against the network allow-list: shell-host rules are
// read-only audit commands from the rules directory, exactly as when they run locally.
func (c *Client) Exec(ctx context.Context, command string) ([]byte, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()
	ctx, cancel := context.WithTimeout(ctx, c.cfg.CommandTimeout)
	defer cancel()

	sess, err := c.ssh.NewSession()
	if err != nil {
		return nil, fmt.Errorf("open session: %w", err)
	}
	defer sess.Close()
	out := &capWriter{limit: c.cfg.MaxOutput, onFull: func() { sess.Close() }}
	sess.Stdout, sess.Stderr = out, out

	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = sess.Signal(ssh.SIGKILL)
		sess.Close()
		<-done
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return out.bytes(), fmt.Errorf("command %w after %s", ErrTimeout, c.cfg.CommandTimeout)
		}
		return out.bytes(), ctx.Err()
	}
	if out.overflow {
		return out.bytes(), fmt.Errorf("%w (%d bytes)", ErrOutputTooLarge, c.cfg.MaxOutput)
	}
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return out.bytes(), nil // some devices close without an exit status
	}
	return out.bytes(), err
}

// ShQuote quotes s for a POSIX shell.
func ShQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// HostCommand wraps an audit command for the interpreter the rule was written for, so it runs
// the same way regardless of the remote user's login shell.
func HostCommand(command, shell string) string {
	switch shell {
	case "powershell":
		return "powershell -NoProfile -NonInteractive -EncodedCommand " + encodePowerShell(command)
	case "bash":
		return "bash -c " + ShQuote(command)
	default:
		return "sh -c " + ShQuote(command)
	}
}

func encodePowerShell(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 0, len(u)*2)
	for _, r := range u {
		b = append(b, byte(r), byte(r>>8))
	}
	return base64.StdEncoding.EncodeToString(b)
}
