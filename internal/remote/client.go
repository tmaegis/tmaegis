// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// PasswordEnv is the environment variable a password can be taken from in non-interactive
// runs. It is discouraged: environment variables leak into process listings, crash dumps and
// CI logs. Prefer ssh-agent or a key file.
const PasswordEnv = "TMAEGIS_SSH_PASSWORD"

// Defaults for a Config left at zero values.
const (
	DefaultConnectTimeout = 15 * time.Second
	DefaultCommandTimeout = 2 * time.Minute
	DefaultMaxOutput      = 16 << 20 // 16 MiB per command
	DefaultMaxSessions    = 4
)

// Prompter asks the operator for a secret. The default reads from the controlling terminal
// with echo off and refuses when there is no terminal. Answers are never stored or logged.
type Prompter func(prompt string) (string, error)

// Config holds connection settings. Nothing in it is ever persisted.
type Config struct {
	KeyFile    string // private key; "" tries ~/.ssh/id_ed25519, id_ecdsa, id_rsa
	KnownHosts string // "" = ~/.ssh/known_hosts
	// InsecureIgnoreHostKey disables host-key verification (man-in-the-middle exposure).
	InsecureIgnoreHostKey bool
	Jump                  *Endpoint // optional bastion, verified and authenticated like the target

	ConnectTimeout time.Duration
	CommandTimeout time.Duration
	MaxOutput      int // bytes per command
	MaxSessions    int // concurrent sessions on the one connection

	// Prompt asks for passphrases and passwords; nil = terminal prompt (TTY only).
	Prompt Prompter
	// Getenv reads the environment (tests); nil = os.Getenv.
	Getenv func(string) string
	// NoDefaultKeys skips ~/.ssh/id_* discovery (tests).
	NoDefaultKeys bool
}

func (c *Config) defaults() {
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = DefaultConnectTimeout
	}
	if c.CommandTimeout <= 0 {
		c.CommandTimeout = DefaultCommandTimeout
	}
	if c.MaxOutput <= 0 {
		c.MaxOutput = DefaultMaxOutput
	}
	if c.MaxSessions <= 0 {
		c.MaxSessions = DefaultMaxSessions
	}
	if c.Getenv == nil {
		c.Getenv = os.Getenv
	}
	if c.Prompt == nil {
		c.Prompt = TerminalPrompt
	}
}

// Client is one authenticated SSH connection; sessions on it are bounded by MaxSessions.
type Client struct {
	ssh      *ssh.Client
	jump     *ssh.Client
	cfg      Config
	endpoint Endpoint
	sem      chan struct{}
	agentCon net.Conn
}

// Errors callers may test for.
var (
	ErrUnknownHost    = errors.New("unknown host key")
	ErrHostKeyChanged = errors.New("host key mismatch")
	ErrTimeout        = errors.New("timed out")
	ErrOutputTooLarge = errors.New("output exceeds the size limit")
	ErrNotAllowed     = errors.New("command not allowed")
)

// Dial connects to ep, verifies its host key and authenticates.
func Dial(ctx context.Context, ep Endpoint, cfg Config) (*Client, error) {
	cfg.defaults()
	c := &Client{cfg: cfg, endpoint: ep, sem: make(chan struct{}, cfg.MaxSessions)}
	var conn net.Conn
	var err error
	if cfg.Jump != nil {
		c.jump, err = c.handshake(ctx, *cfg.Jump, nil)
		if err != nil {
			c.closeAgent()
			return nil, fmt.Errorf("jump host %s: %w", cfg.Jump, err)
		}
		conn, err = dialVia(ctx, c.jump, ep.Addr(), cfg.ConnectTimeout)
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("via jump host %s: %w", cfg.Jump, err)
		}
	}
	c.ssh, err = c.handshake(ctx, ep, conn)
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func dialVia(ctx context.Context, jump *ssh.Client, addr string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type res struct {
		c   net.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() { c, err := jump.Dial("tcp", addr); ch <- res{c, err} }()
	select {
	case r := <-ch:
		return r.c, r.err
	case <-ctx.Done():
		go func() {
			if r := <-ch; r.c != nil {
				r.c.Close()
			}
		}()
		return nil, fmt.Errorf("connect to %s: %w", addr, ErrTimeout)
	}
}

// handshake opens (or reuses conn) and runs the SSH handshake with ep within ConnectTimeout.
func (c *Client) handshake(ctx context.Context, ep Endpoint, conn net.Conn) (*ssh.Client, error) {
	hostKey, err := c.hostKeyCallback()
	if err != nil {
		return nil, err
	}
	auths, err := c.authMethods(ep)
	if err != nil {
		return nil, err
	}
	sc := &ssh.ClientConfig{
		User:            ep.User,
		Auth:            auths,
		HostKeyCallback: hostKey,
		Timeout:         c.cfg.ConnectTimeout,
	}
	if !c.cfg.InsecureIgnoreHostKey {
		// Offer the algorithms already recorded for this host first, so an existing ed25519
		// entry is not reported as a mismatch when the server prefers its RSA key.
		sc.HostKeyAlgorithms = c.knownAlgorithms(ep)
	}
	if conn == nil {
		d := net.Dialer{Timeout: c.cfg.ConnectTimeout}
		conn, err = d.DialContext(ctx, "tcp", ep.Addr())
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil, fmt.Errorf("connect to %s: %w", ep.Addr(), ErrTimeout)
			}
			return nil, fmt.Errorf("connect to %s: %w", ep.Addr(), err)
		}
	}
	// The handshake itself must not hang on a silent server.
	_ = conn.SetDeadline(time.Now().Add(c.cfg.ConnectTimeout))
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	cc, chans, reqs, err := ssh.NewClientConn(conn, ep.Addr(), sc)
	stop()
	if err != nil {
		conn.Close()
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return nil, fmt.Errorf("ssh handshake with %s: %w", ep.Addr(), ErrTimeout)
		}
		if errors.Is(err, ErrUnknownHost) || errors.Is(err, ErrHostKeyChanged) {
			return nil, unwrapHostKeyErr(err)
		}
		return nil, fmt.Errorf("ssh %s: %w", ep, err)
	}
	_ = conn.SetDeadline(time.Time{})
	return ssh.NewClient(cc, chans, reqs), nil
}

// hostKeyError carries the full operator guidance; ssh wraps callback errors in its own text.
type hostKeyError struct {
	kind error
	msg  string
}

func (e *hostKeyError) Error() string { return e.msg }
func (e *hostKeyError) Unwrap() error { return e.kind }

func unwrapHostKeyErr(err error) error {
	var hk *hostKeyError
	if errors.As(err, &hk) {
		return hk
	}
	return err
}

func (c *Client) knownHostsPath() string {
	if c.cfg.KnownHosts != "" {
		return c.cfg.KnownHosts
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "known_hosts")
}

func (c *Client) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if c.cfg.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // explicit, loudly warned opt-out
	}
	path := c.knownHostsPath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// No file yet: every host is unknown; still report its fingerprint.
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return unknownHostErr(hostname, path, key)
		}, nil
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("known_hosts %s: %w", path, err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		var ke *knownhosts.KeyError
		if errors.As(err, &ke) {
			if len(ke.Want) == 0 {
				return unknownHostErr(hostname, path, key)
			}
			var lines []string
			for _, w := range ke.Want {
				lines = append(lines, fmt.Sprintf("%s:%d", w.Filename, w.Line))
			}
			return &hostKeyError{kind: ErrHostKeyChanged, msg: fmt.Sprintf(
				"HOST KEY MISMATCH for %s: the server presented %s %s, which differs from the key recorded at %s. "+
					"Someone may be intercepting the connection (man-in-the-middle), or the host was reinstalled. "+
					"Verify the new fingerprint out of band before updating known_hosts (ssh-keygen -R %s).",
				hostname, key.Type(), ssh.FingerprintSHA256(key), strings.Join(lines, ", "), knownhosts.Normalize(hostname))}
		}
		return err
	}, nil
}

func unknownHostErr(hostname, path string, key ssh.PublicKey) error {
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		host, port = hostname, "22"
	}
	scan := "ssh-keyscan"
	if port != "22" {
		scan += " -p " + port
	}
	return &hostKeyError{kind: ErrUnknownHost, msg: fmt.Sprintf(
		"unknown host %s: its %s key fingerprint is %s and it is not in %s. "+
			"Verify the fingerprint out of band, then add it, e.g.: %s %s >> %s (or connect once with ssh and accept it).",
		hostname, key.Type(), ssh.FingerprintSHA256(key), path, scan, host, path)}
}

// knownAlgorithms lists the key types known_hosts holds for ep (nil = library default).
func (c *Client) knownAlgorithms(ep Endpoint) []string {
	path := c.knownHostsPath()
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil
	}
	// Asking with a throwaway key makes knownhosts list the recorded keys for the host.
	probe := probeKey{}
	var ke *knownhosts.KeyError
	if err := cb(ep.Addr(), &net.TCPAddr{}, probe); !errors.As(err, &ke) || len(ke.Want) == 0 {
		return nil
	}
	var algos []string
	seen := map[string]bool{}
	for _, w := range ke.Want {
		for _, a := range algorithmsFor(w.Key.Type()) {
			if !seen[a] {
				seen[a] = true
				algos = append(algos, a)
			}
		}
	}
	return algos
}

func algorithmsFor(keyType string) []string {
	if keyType == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	}
	return []string{keyType}
}

type probeKey struct{}

func (probeKey) Type() string                        { return "tmaegis-probe" }
func (probeKey) Marshal() []byte                     { return []byte("tmaegis-probe") }
func (probeKey) Verify([]byte, *ssh.Signature) error { return errors.New("probe") }

// authMethods: ssh-agent, then the key file (or default keys), then password and
// keyboard-interactive (TTY prompt or TMAEGIS_SSH_PASSWORD).
func (c *Client) authMethods(ep Endpoint) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	var signers []ssh.Signer
	if sock := c.cfg.Getenv("SSH_AUTH_SOCK"); sock != "" && c.agentCon == nil {
		if conn, err := net.Dial("unix", sock); err == nil {
			c.agentCon = conn
		}
	}
	if c.agentCon != nil {
		if s, err := agent.NewClient(c.agentCon).Signers(); err == nil {
			signers = append(signers, s...)
		}
	}
	keys := []string{}
	explicit := c.cfg.KeyFile != ""
	if explicit {
		keys = append(keys, c.cfg.KeyFile)
	} else if !c.cfg.NoDefaultKeys {
		if home, err := os.UserHomeDir(); err == nil {
			for _, n := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				keys = append(keys, filepath.Join(home, ".ssh", n))
			}
		}
	}
	for _, k := range keys {
		s, err := c.loadKey(k)
		if err != nil {
			if explicit {
				return nil, err
			}
			continue // default keys are best effort
		}
		signers = append(signers, s)
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	pw := passwordSource{cfg: &c.cfg, target: ep}
	methods = append(methods,
		ssh.RetryableAuthMethod(ssh.PasswordCallback(pw.password), 1),
		ssh.RetryableAuthMethod(ssh.KeyboardInteractive(pw.challenge), 1))
	return methods, nil
}

func (c *Client) loadKey(path string) (ssh.Signer, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ssh key %s: %w", path, err)
	}
	s, err := ssh.ParsePrivateKey(pem)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		pass, perr := c.cfg.Prompt(fmt.Sprintf("Passphrase for %s: ", path))
		if perr != nil {
			return nil, fmt.Errorf("ssh key %s is encrypted: %w", path, perr)
		}
		s, err = ssh.ParsePrivateKeyWithPassphrase(pem, []byte(pass))
	}
	if err != nil {
		return nil, fmt.Errorf("ssh key %s: %w", path, err)
	}
	return s, nil
}

// passwordSource answers password and keyboard-interactive prompts, from the environment
// variable or the terminal. The secret lives only for the duration of the call.
type passwordSource struct {
	cfg    *Config
	target Endpoint
}

func (p passwordSource) password() (string, error) {
	if v := p.cfg.Getenv(PasswordEnv); v != "" {
		return v, nil
	}
	return p.cfg.Prompt(fmt.Sprintf("Password for %s: ", p.target))
}

func (p passwordSource) challenge(name, instruction string, questions []string, echos []bool) ([]string, error) {
	answers := make([]string, len(questions))
	env := p.cfg.Getenv(PasswordEnv)
	for i, q := range questions {
		if env != "" && len(questions) == 1 && !echos[i] {
			answers[i] = env
			continue
		}
		if echos[i] {
			// Visible questions (e.g. "Username:") are not secrets but still need a terminal.
			a, err := p.cfg.Prompt(q)
			if err != nil {
				return nil, err
			}
			answers[i] = a
			continue
		}
		a, err := p.cfg.Prompt(fmt.Sprintf("%s (%s): ", strings.TrimSpace(q), p.target))
		if err != nil {
			return nil, err
		}
		answers[i] = a
	}
	return answers, nil
}

// Endpoint is the destination this client is connected to.
func (c *Client) Endpoint() Endpoint { return c.endpoint }

func (c *Client) closeAgent() {
	if c.agentCon != nil {
		c.agentCon.Close()
		c.agentCon = nil
	}
}

// Close ends the connection (and the jump connection).
func (c *Client) Close() error {
	var err error
	if c.ssh != nil {
		err = c.ssh.Close()
	}
	if c.jump != nil {
		c.jump.Close()
	}
	c.closeAgent()
	return err
}
