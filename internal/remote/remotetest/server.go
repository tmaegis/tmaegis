// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package remotetest runs an in-process SSH server for tests: a fake Unix host answering a
// few exec commands, and a fake network device with an interactive CLI and a pager. It never
// touches the network beyond 127.0.0.1.
package remotetest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ExecFunc answers one exec request: output, exit status, and a delay before answering.
type ExecFunc func(cmd string) (out string, exit int, delay time.Duration)

// Server is a running fake SSH server.
type Server struct {
	Host     string
	Port     int
	HostKey  ssh.Signer
	Client   ssh.Signer // the key the server accepts
	Password string     // accepted for password and keyboard-interactive auth ("" = none)

	Exec   ExecFunc    // nil = exec requests fail
	Device *FakeDevice // non-nil = shell requests drive this device

	ln        net.Listener
	mu        sync.Mutex
	execs     []string
	forwarded int
	closed    chan struct{}
}

// Options configure Start.
type Options struct {
	Password string
	Exec     ExecFunc
	Device   *FakeDevice
}

// Start listens on 127.0.0.1 and serves until the test ends.
func Start(t testing.TB, o Options) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{HostKey: NewSigner(t), Client: NewSigner(t), Password: o.Password, Exec: o.Exec, Device: o.Device,
		ln: ln, closed: make(chan struct{})}
	addr := ln.Addr().(*net.TCPAddr)
	s.Host, s.Port = "127.0.0.1", addr.Port
	go s.serve()
	t.Cleanup(s.Close)
	return s
}

// Addr is host:port.
func (s *Server) Addr() string { return net.JoinHostPort(s.Host, strconv.Itoa(s.Port)) }

// Close stops the server.
func (s *Server) Close() {
	select {
	case <-s.closed:
	default:
		close(s.closed)
		s.ln.Close()
	}
}

// Execs returns every exec command received.
func (s *Server) Execs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.execs...)
}

// NewSigner returns a fresh ed25519 signer.
func NewSigner(t testing.TB) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// WriteKnownHosts writes a known_hosts file trusting key for this server and returns its path.
func (s *Server) WriteKnownHosts(t testing.TB, key ssh.PublicKey) string {
	t.Helper()
	if key == nil {
		key = s.HostKey.PublicKey()
	}
	p := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(s.Addr())}, key)
	if err := os.WriteFile(p, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// WriteClientKey writes the accepted client key in OpenSSH format (encrypted when passphrase
// is set) and returns its path.
func (s *Server) WriteClientKey(t testing.TB, passphrase string) string {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	s.Client = signer
	var blk *pem.Block
	var err error
	if passphrase == "" {
		blk, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		blk, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(p, pem.EncodeToMemory(blk), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func (s *Server) serve() {
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(s.Client.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown key")
		},
		PasswordCallback: func(_ ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if s.Password != "" && string(pw) == s.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("bad password")
		},
		KeyboardInteractiveCallback: func(_ ssh.ConnMetadata, ch ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			ans, err := ch("", "", []string{"Password: "}, []bool{false})
			if err == nil && s.Password != "" && len(ans) == 1 && ans[0] == s.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("bad password")
		},
	}
	cfg.AddHostKey(s.HostKey)
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c, cfg)
	}
}

func (s *Server) handle(c net.Conn, cfg *ssh.ServerConfig) {
	defer c.Close()
	sc, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() == "direct-tcpip" {
			go s.forward(nc) // jump-host (ProxyJump) support
			continue
		}
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go s.session(ch, creqs)
	}
}

// Forwarded counts direct-tcpip channels served (jump-host use).
func (s *Server) Forwarded() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forwarded
}

func (s *Server) forward(nc ssh.NewChannel) {
	var req struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(nc.ExtraData(), &req); err != nil || req.Host != "127.0.0.1" {
		nc.Reject(ssh.Prohibited, "only 127.0.0.1")
		return
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port))))
	if err != nil {
		nc.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := nc.Accept()
	if err != nil {
		conn.Close()
		return
	}
	s.mu.Lock()
	s.forwarded++
	s.mu.Unlock()
	go ssh.DiscardRequests(reqs)
	go func() { _, _ = io.Copy(conn, ch); conn.Close() }()
	_, _ = io.Copy(ch, conn)
	ch.Close()
}

func (s *Server) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "pty-req", "env":
			req.Reply(true, nil)
		case "exec":
			if len(req.Payload) < 4 {
				req.Reply(false, nil)
				return
			}
			n := binary.BigEndian.Uint32(req.Payload)
			cmd := string(req.Payload[4 : 4+n])
			s.mu.Lock()
			s.execs = append(s.execs, cmd)
			s.mu.Unlock()
			if s.Exec == nil {
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			out, code, delay := s.Exec(cmd)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-s.closed:
					return
				}
			}
			_, _ = ch.Write([]byte(out))
			status := make([]byte, 4)
			binary.BigEndian.PutUint32(status, uint32(code))
			_, _ = ch.SendRequest("exit-status", false, status)
			return
		case "shell":
			if s.Device == nil {
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			go func() {
				for r := range reqs {
					if r.WantReply {
						r.Reply(false, nil)
					}
				}
			}()
			s.Device.serve(ch)
			return
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// FakeHost answers the exec commands a remote host scan sends.
type FakeHost struct {
	Uname    string            // "uname -s; uname -m; uname -n" output (default Linux/x86_64/web01)
	Files    map[string]string // path -> content (cat and test -e)
	Commands map[string]string // audit command (as written in the rule) -> output, exit 0
}

// Handle implements ExecFunc.
func (h *FakeHost) Handle(cmd string) (string, int, time.Duration) {
	switch {
	case cmd == "uname -s; uname -m; uname -n":
		if h.Uname != "" {
			return h.Uname, 0, 0
		}
		return "Linux\nx86_64\nweb01\n", 0, 0
	case strings.HasPrefix(cmd, "cat /etc/os-release"):
		if c, ok := h.Files["/etc/os-release"]; ok {
			return c, 0, 0
		}
		return "cat: /etc/os-release: No such file or directory\n", 1, 0
	case strings.HasPrefix(cmd, "cat -- "):
		p := Unquote(strings.TrimPrefix(cmd, "cat -- "))
		if c, ok := h.Files[p]; ok {
			return c, 0, 0
		}
		return "cat: " + p + ": No such file or directory\n", 1, 0
	case strings.HasPrefix(cmd, "test -e "):
		if _, ok := h.Files[Unquote(strings.TrimPrefix(cmd, "test -e "))]; ok {
			return "", 0, 0
		}
		return "", 1, 0
	case strings.HasPrefix(cmd, "sh -c "), strings.HasPrefix(cmd, "bash -c "):
		_, q, _ := strings.Cut(cmd, " -c ")
		if out, ok := h.Commands[Unquote(q)]; ok {
			return out, 0, 0
		}
		return "sh: command not found\n", 127, 0
	}
	return "unknown command\n", 127, 0
}

// Unquote reverses POSIX single quoting ('a'\”b' -> a'b).
func Unquote(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			in = !in
		case c == '\\' && !in && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// FakeDevice is an interactive network-device CLI: it echoes input, prints a prompt, pages
// output every PageLines lines with " --More-- " until "terminal length 0" turns the pager off.
type FakeDevice struct {
	Prompt    string            // e.g. "R1#"
	Banner    string            // printed before the first prompt
	PageLines int               // 0 = no pager
	Commands  map[string]string // command -> output
	// PagerOff lists commands that disable the pager (default "terminal length 0").
	PagerOff []string

	mu       sync.Mutex
	received []string
}

// Received returns every line the client sent.
func (d *FakeDevice) Received() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.received...)
}

func (d *FakeDevice) serve(ch ssh.Channel) {
	write := func(s string) { _, _ = ch.Write([]byte(s)) }
	pager := d.PageLines
	off := d.PagerOff
	if off == nil {
		off = []string{"terminal length 0"}
	}
	if d.Banner != "" {
		write(strings.ReplaceAll(d.Banner, "\n", "\r\n") + "\r\n")
	}
	write(d.Prompt)
	buf := make([]byte, 1)
	var line []byte
	readByte := func() (byte, bool) {
		if _, err := ch.Read(buf); err != nil {
			return 0, false
		}
		return buf[0], true
	}
	for {
		c, ok := readByte()
		if !ok {
			return
		}
		if c != '\r' && c != '\n' {
			line = append(line, c)
			write(string(c)) // pty echo
			continue
		}
		cmd := strings.TrimSpace(string(line))
		line = line[:0]
		write("\r\n")
		if cmd == "" {
			write(d.Prompt)
			continue
		}
		d.mu.Lock()
		d.received = append(d.received, cmd)
		d.mu.Unlock()
		if cmd == "exit" {
			return
		}
		isOff := false
		for _, o := range off {
			if cmd == o {
				isOff = true
			}
		}
		if isOff {
			pager = 0
			write(d.Prompt)
			continue
		}
		out, known := d.Commands[cmd]
		if !known {
			write("% Invalid input detected at '^' marker.\r\n" + d.Prompt)
			continue
		}
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		for i, l := range lines {
			if pager > 0 && i > 0 && i%pager == 0 {
				write(" --More-- ")
				if b, ok := readByte(); !ok {
					return
				} else if b == 'q' {
					break
				}
				write(strings.Repeat("\b", 10) + strings.Repeat(" ", 10) + strings.Repeat("\b", 10))
			}
			write(l + "\r\n")
		}
		write(d.Prompt)
	}
}

// Silent listens on 127.0.0.1 and accepts connections without ever speaking SSH (for
// timeout tests). It returns the address.
func Silent(t testing.TB) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var conns []net.Conn
	var mu sync.Mutex
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
	})
	return ln.Addr().String()
}
