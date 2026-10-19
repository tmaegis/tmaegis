// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/remote/remotetest"
	"github.com/tmaegis/tmaegis/internal/rules"
)

func TestMain(m *testing.M) {
	shellQuiet, shellPromptNudge, shellLoosePrompt = 40*time.Millisecond, 300*time.Millisecond, 300*time.Millisecond
	os.Exit(m.Run())
}

func noTTY(string) (string, error) { return "", ErrNoTerminal }

// testConfig trusts srv, uses its client key and ignores the real environment.
func testConfig(t *testing.T, srv *remotetest.Server) Config {
	t.Helper()
	return Config{
		KeyFile:        srv.WriteClientKey(t, ""),
		KnownHosts:     srv.WriteKnownHosts(t, nil),
		ConnectTimeout: 3 * time.Second,
		CommandTimeout: 3 * time.Second,
		Prompt:         noTTY,
		Getenv:         func(string) string { return "" },
		NoDefaultKeys:  true,
	}
}

func endpoint(srv *remotetest.Server) Endpoint {
	return Endpoint{User: "auditor", Host: srv.Host, Port: srv.Port}
}

func linuxHost() *remotetest.FakeHost {
	return &remotetest.FakeHost{
		Files: map[string]string{
			"/etc/os-release":      "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n",
			"/etc/ssh/sshd_config": "PermitRootLogin no\nX11Forwarding yes\n",
		},
		Commands: map[string]string{"sysctl -n net.ipv4.ip_forward": "0\n"},
	}
}

func TestParseEndpointAndInventory(t *testing.T) {
	cases := map[string]Endpoint{
		"root@db1":            {User: "root", Host: "db1", Port: 22},
		"me@10.0.0.1:2222":    {User: "me", Host: "10.0.0.1", Port: 2222},
		"me@[2001:db8::1]:23": {User: "me", Host: "2001:db8::1", Port: 23},
		"me@2001:db8::1":      {User: "me", Host: "2001:db8::1", Port: 22},
		"switch":              {User: "dflt", Host: "switch", Port: 22},
	}
	for in, want := range cases {
		got, err := ParseEndpoint(in, "dflt")
		if err != nil || got != want {
			t.Errorf("ParseEndpoint(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "me@", "me@host:0", "me@host:x", "a b@host"} {
		if _, err := ParseEndpoint(bad, "dflt"); err == nil {
			t.Errorf("ParseEndpoint(%q) should fail", bad)
		}
	}
	if _, err := ParseEndpoint("host", ""); err == nil {
		t.Error("missing user must fail")
	}
	inv, err := ParseInventory(strings.NewReader("# fleet\nweb1 linux\n\nadmin@core1:2022 cisco-ios # core\n"), "ops")
	if err != nil || len(inv) != 2 || inv[0].Endpoint.User != "ops" || inv[0].Target != "linux" ||
		inv[1].Endpoint.Port != 2022 || inv[1].Target != "cisco-ios" || inv[1].Line != 4 {
		t.Fatalf("inventory: %+v %v", inv, err)
	}
	if _, err := ParseInventory(strings.NewReader("a b c\n"), "ops"); err == nil {
		t.Error("too many fields must fail")
	}
	if _, err := ParseInventory(strings.NewReader("# empty\n"), "ops"); err == nil {
		t.Error("empty inventory must fail")
	}
}

func TestCheckCommandAllowList(t *testing.T) {
	ok := []string{"show running-config", "terminal length 0", "show configuration | display set | no-more",
		"show full-configuration", "get system status", "SHOW RUNNING-CONFIG", "show config running",
		"set cli config-output-format set", "show running-config | include snmp", "cat /conf/config.xml",
		"show configuration", "no page", "export configuration"}
	for _, c := range ok {
		if err := CheckCommand(c); err != nil {
			t.Errorf("%q rejected: %v", c, err)
		}
	}
	bad := []string{"configure terminal", "conf t", "write memory", "wr", "copy running-config startup-config",
		"reload", "commit", "delete flash:x", "erase startup-config", "set system host-name x", "show run; reload",
		"show run && reboot", "show run\nreload", "show run | tee x", "show run > x", "show $(reboot)", "request system reboot",
		"execute reboot", "show run | display set | commit", "show run `reboot`", "terminal length 0 ; write",
		"enable", "debug all", "clear counters", "show running-config | append x", "set cli pager on now", "",
		"cat /conf/config.xml | sh", "show run | display set xml commit"}
	for _, c := range bad {
		if err := CheckCommand(c); err == nil || !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q accepted", c)
		}
	}
}

func TestVendorTable(t *testing.T) {
	for target, v := range Vendors {
		if v.Target != target || v.Name == "" || (v.Mode != ModeShell && v.Mode != ModeExec) {
			t.Errorf("%s: incomplete entry %+v", target, v)
		}
		for _, c := range append(append([]string{}, v.Setup...), v.Commands...) {
			if err := CheckCommand(c); err != nil {
				t.Errorf("%s: table command fails its own guard: %v", target, err)
			}
		}
		_, err := PlanFor(target, nil)
		if v.Verified && err != nil {
			t.Errorf("%s: verified vendor must plan without --ssh-command: %v", target, err)
		}
		if !v.Verified && (err == nil || !strings.Contains(err.Error(), "--ssh-command")) {
			t.Errorf("%s: unverified vendor must require --ssh-command, got %v", target, err)
		}
		if _, err := PlanFor(target, []string{"show running-config"}); err != nil {
			t.Errorf("%s: explicit command: %v", target, err)
		}
	}
	// The rules directory's device targets must all be covered.
	entries, _ := os.ReadDir("../../rules/network")
	for _, e := range entries {
		if e.IsDir() && !IsDevice(e.Name()) {
			t.Errorf("rules/network/%s has no vendor table entry", e.Name())
		}
	}
	if _, err := PlanFor("cisco-ios", []string{"configure terminal"}); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("user command must pass the guard: %v", err)
	}
	if _, err := PlanFor("paloalto", []string{"set cli pager off"}); err == nil {
		t.Error("only session settings must be rejected")
	}
	p, err := PlanFor("paloalto", []string{"set cli pager off", "set cli config-output-format set", "show config running"})
	if err != nil || len(p.Setup) != 2 || len(p.Commands) != 1 {
		t.Errorf("paloalto plan: %+v %v", p, err)
	}
	if _, err := PlanFor("linux", nil); err == nil {
		t.Error("linux is not a device")
	}
}

func TestDialAndExec(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	c, err := Dial(context.Background(), endpoint(srv), testConfig(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.Exec(context.Background(), "cat -- "+ShQuote("/etc/ssh/sshd_config"))
	if err != nil || !strings.Contains(string(out), "PermitRootLogin no") {
		t.Fatalf("exec: %q %v", out, err)
	}
	_, err = c.Exec(context.Background(), "false-command")
	var exit *ssh.ExitError
	if !errors.As(err, &exit) || exit.ExitStatus() != 127 {
		t.Fatalf("exit status: %v", err)
	}
}

func TestUnknownHost(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	cfg := testConfig(t, srv)
	other := remotetest.Start(t, remotetest.Options{})
	cfg.KnownHosts = other.WriteKnownHosts(t, nil) // trusts a different address only
	_, err := Dial(context.Background(), endpoint(srv), cfg)
	if !errors.Is(err, ErrUnknownHost) {
		t.Fatalf("want unknown host, got %v", err)
	}
	fp := ssh.FingerprintSHA256(srv.HostKey.PublicKey())
	if !strings.Contains(err.Error(), fp) || !strings.Contains(err.Error(), "ssh-keyscan -p") {
		t.Errorf("error must show the fingerprint and how to add it: %v", err)
	}
	cfg.KnownHosts = filepath.Join(t.TempDir(), "missing")
	if _, err := Dial(context.Background(), endpoint(srv), cfg); !errors.Is(err, ErrUnknownHost) || !strings.Contains(err.Error(), fp) {
		t.Fatalf("missing known_hosts: %v", err)
	}
	cfg.InsecureIgnoreHostKey = true
	c, err := Dial(context.Background(), endpoint(srv), cfg)
	if err != nil {
		t.Fatalf("insecure mode must connect: %v", err)
	}
	c.Close()
}

func TestHostKeyMismatch(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	cfg := testConfig(t, srv)
	cfg.KnownHosts = srv.WriteKnownHosts(t, remotetest.NewSigner(t).PublicKey())
	_, err := Dial(context.Background(), endpoint(srv), cfg)
	if !errors.Is(err, ErrHostKeyChanged) || !strings.Contains(err.Error(), "HOST KEY MISMATCH") {
		t.Fatalf("want mismatch, got %v", err)
	}
	if len(srv.Execs()) != 0 {
		t.Fatal("nothing may run on a host whose key does not match")
	}
}

func TestAuth(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle, Password: "s3cret"})
	cfg := testConfig(t, srv)
	good := cfg.KeyFile
	srv.WriteClientKey(t, "") // the server now expects a different key
	cfg.KeyFile = good
	// Without a terminal the password methods cannot be answered: say so.
	_, err := Dial(context.Background(), endpoint(srv), cfg)
	if err == nil || !strings.Contains(err.Error(), "not a terminal") {
		t.Fatalf("want no-terminal failure, got %v", err)
	}
	// A wrong password is a plain authentication failure.
	cfg.Prompt = func(string) (string, error) { return "wrong", nil }
	_, err = Dial(context.Background(), endpoint(srv), cfg)
	if err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("want auth failure, got %v", err)
	}
	if strings.Contains(err.Error(), "wrong") || strings.Contains(err.Error(), "s3cret") {
		t.Fatal("secret leaked into the error")
	}
	cfg.Prompt = noTTY
	// Password from the (discouraged) environment variable.
	cfg.Getenv = func(k string) string {
		if k == PasswordEnv {
			return "s3cret"
		}
		return ""
	}
	c, err := Dial(context.Background(), endpoint(srv), cfg)
	if err != nil {
		t.Fatalf("env password: %v", err)
	}
	c.Close()
	// Password typed at the prompt.
	cfg.Getenv = func(string) string { return "" }
	asked := 0
	cfg.Prompt = func(p string) (string, error) { asked++; return "s3cret", nil }
	c, err = Dial(context.Background(), endpoint(srv), cfg)
	if err != nil || asked == 0 {
		t.Fatalf("prompted password: %v (asked %d)", err, asked)
	}
	c.Close()
	// Encrypted key: passphrase prompt.
	cfg.KeyFile = srv.WriteClientKey(t, "pass phrase")
	cfg.Prompt = func(p string) (string, error) {
		if strings.HasPrefix(p, "Passphrase for") {
			return "pass phrase", nil
		}
		return "", errors.New("unexpected prompt " + p)
	}
	c, err = Dial(context.Background(), endpoint(srv), cfg)
	if err != nil {
		t.Fatalf("encrypted key: %v", err)
	}
	c.Close()
	cfg.Prompt = noTTY
	if _, err := Dial(context.Background(), endpoint(srv), cfg); !errors.Is(err, ErrNoTerminal) {
		t.Fatalf("encrypted key without a terminal: %v", err)
	}
}

func TestTimeouts(t *testing.T) {
	addr := remotetest.Silent(t)
	host, port, _ := strings.Cut(addr, ":")
	ep, _ := ParseEndpoint("u@"+host+":"+port, "")
	start := time.Now()
	_, err := Dial(context.Background(), ep, Config{ConnectTimeout: 300 * time.Millisecond, InsecureIgnoreHostKey: true,
		Prompt: noTTY, Getenv: func(string) string { return "" }, NoDefaultKeys: true})
	if !errors.Is(err, ErrTimeout) || time.Since(start) > 3*time.Second {
		t.Fatalf("handshake timeout: %v after %s", err, time.Since(start))
	}

	srv := remotetest.Start(t, remotetest.Options{Exec: func(cmd string) (string, int, time.Duration) {
		if cmd == "slow" {
			return "late", 0, 10 * time.Second
		}
		return strings.Repeat("A", 100_000), 0, 0
	}})
	cfg := testConfig(t, srv)
	cfg.CommandTimeout = 300 * time.Millisecond
	cfg.MaxOutput = 1024
	c, err := Dial(context.Background(), endpoint(srv), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Exec(context.Background(), "slow"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("command timeout: %v", err)
	}
	out, err := c.Exec(context.Background(), "big")
	if !errors.Is(err, ErrOutputTooLarge) || len(out) > 1024 {
		t.Fatalf("output cap: %d bytes, %v", len(out), err)
	}
	// The connection is still usable after both.
	if _, err := c.Exec(context.Background(), "big2"); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("connection unusable: %v", err)
	}
}

func TestDetectHost(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	c, err := Dial(context.Background(), endpoint(srv), testConfig(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	h, tg := DetectHost(context.Background(), c)
	if h.OS != "linux" || h.Distro != "ubuntu" || h.Family != "ubuntu-22.04" || tg.Hostname != "web01" || tg.Arch != "amd64" || tg.OS != "linux" {
		t.Fatalf("detected %+v %+v", h, tg)
	}
}

func writeRule(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteHostScan(t *testing.T) {
	dir := t.TempDir()
	writeRule(t, filepath.Join(dir, "linux"), "a.yaml", `id: T.1
title: root login disabled
target: linux
level: 1
severity: high
audit: {type: file_regex, file_path: /etc/ssh/sshd_config, pattern: '(?m)^PermitRootLogin no'}
remediation: {type: command, command: "echo never-run"}
`)
	writeRule(t, filepath.Join(dir, "linux"), "b.yaml", `id: T.2
title: x11 forwarding off
target: linux
level: 1
severity: medium
audit: {type: file_regex, file_path: /etc/ssh/sshd_config, pattern: '(?m)^X11Forwarding no'}
`)
	writeRule(t, filepath.Join(dir, "linux"), "c.yaml", `id: T.3
title: ip forwarding off
target: linux
level: 1
severity: medium
audit: {type: command, command: sysctl -n net.ipv4.ip_forward, expected_output: "0"}
`)
	writeRule(t, filepath.Join(dir, "linux"), "d.yaml", `id: T.4
title: other distro only
target: linux
os_family: [rhel-9]
level: 1
severity: medium
audit: {type: command, command: "true"}
`)
	writeRule(t, filepath.Join(dir, "linux"), "e.yaml", `id: T.5
title: needs kube
target: linux
level: 1
severity: low
audit: {type: command, command: "true", requires_file: [/etc/kubernetes/admin.conf]}
`)
	srv := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	c, err := Dial(context.Background(), endpoint(srv), testConfig(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	h, info := DetectHost(context.Background(), c)
	rs, err := rules.LoadFiltered(dir, rules.Filter{Targets: []string{"linux"}, MaxLevel: 1, OSFamily: h.Family})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 4 {
		t.Fatalf("os-release must filter out the rhel-9 rule: %d rules", len(rs))
	}
	run := engine.NewRunnerWithConcurrency(4)
	run.Exec = &HostExecutor{Client: c, Info: info}
	rep := run.RunAudit(context.Background(), rs)
	got := map[string]engine.Status{}
	for _, r := range rep.Results {
		got[r.Rule.ID] = r.Status
	}
	want := map[string]engine.Status{"T.1": engine.StatusPass, "T.2": engine.StatusFail, "T.3": engine.StatusPass, "T.5": engine.StatusSkip}
	for id, st := range want {
		if got[id] != st {
			t.Errorf("%s = %s, want %s", id, got[id], st)
		}
	}
	if rep.Hostname != "web01" || rep.OS != "linux" {
		t.Errorf("report header %q %q", rep.Hostname, rep.OS)
	}
	if err := run.RunRemediation(context.Background(), rs, false); err == nil {
		t.Fatal("remediation over SSH must be refused")
	}
	for _, e := range srv.Execs() {
		if strings.Contains(e, "never-run") {
			t.Fatal("a remediation command reached the host")
		}
	}
}

const runningConfig = `hostname R1
!
aaa new-model
service password-encryption
no ip http server
line vty 0 4
 transport input ssh
 exec-timeout 10 0
!
logging host 10.0.0.5
ntp server 10.0.0.1
snmp-server group G v3 priv
banner login ^C Authorised access only ^C
end`

func TestDeviceShellWithPager(t *testing.T) {
	for _, pagerOff := range []bool{true, false} {
		dev := &remotetest.FakeDevice{Prompt: "R1#", Banner: "User Access Verification", PageLines: 4,
			Commands: map[string]string{"show running-config": runningConfig}}
		if !pagerOff {
			dev.PagerOff = []string{"never"} // the pager stays on: tmaegis must page through it
			dev.Commands["terminal length 0"] = ""
		}
		srv := remotetest.Start(t, remotetest.Options{Device: dev})
		c, err := Dial(context.Background(), endpoint(srv), testConfig(t, srv))
		if err != nil {
			t.Fatal(err)
		}
		plan, err := PlanFor("cisco-ios", nil)
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.FetchConfig(context.Background(), plan)
		c.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(out)); got != runningConfig {
			t.Fatalf("pagerOff=%v config mismatch:\n%q\nwant\n%q", pagerOff, got, runningConfig)
		}
		time.Sleep(50 * time.Millisecond)
		rec := dev.Received()
		if len(rec) < 2 || rec[0] != "terminal length 0" || rec[1] != "show running-config" {
			t.Fatalf("sent %q", rec)
		}
	}
}

func TestDeviceShellGuardAndCap(t *testing.T) {
	dev := &remotetest.FakeDevice{Prompt: "fw01 # ", Commands: map[string]string{
		"show full-configuration": runningConfig, "show big": strings.Repeat("line of text\n", 500)}}
	srv := remotetest.Start(t, remotetest.Options{Device: dev})
	cfg := testConfig(t, srv)
	cfg.MaxOutput = 2048
	c, err := Dial(context.Background(), endpoint(srv), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	sh, err := c.OpenShell(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer sh.Close()
	if sh.Prompt() != "fw01 #" {
		t.Errorf("prompt %q", sh.Prompt())
	}
	for _, bad := range []string{"configure terminal", "execute reboot", "config system global"} {
		if _, err := sh.Run(context.Background(), bad); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("%q not rejected: %v", bad, err)
		}
	}
	out, err := sh.Run(context.Background(), "show full-configuration")
	if err != nil || strings.TrimSpace(string(out)) != runningConfig {
		t.Fatalf("show: %q %v", out, err)
	}
	if _, err := sh.Run(context.Background(), "show big"); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("cap: %v", err)
	}
	for _, r := range dev.Received() {
		if strings.Contains(r, "conf") && r != "show full-configuration" || strings.Contains(r, "execute") {
			t.Fatalf("a rejected command reached the device: %q", r)
		}
	}
}

func TestDeviceExecMode(t *testing.T) {
	srv := remotetest.Start(t, remotetest.Options{Exec: func(cmd string) (string, int, time.Duration) {
		if cmd == "cat /conf/config.xml" {
			return "<pfsense><system><hostname>fw</hostname></system></pfsense>\n", 0, 0
		}
		return "", 1, 0
	}})
	c, err := Dial(context.Background(), endpoint(srv), testConfig(t, srv))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := PlanFor("pfsense", nil); err == nil {
		t.Fatal("pfsense is unverified")
	}
	p, err := PlanFor("pfsense", []string{"cat /conf/config.xml"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.FetchConfig(context.Background(), p)
	if err != nil || !strings.Contains(string(out), "<hostname>fw</hostname>") {
		t.Fatalf("exec fetch: %q %v", out, err)
	}
}

func TestCleanTerminal(t *testing.T) {
	in := "line1\r\n --More-- \b\b\b\b\b\b\b\b\b\b          \b\b\b\b\b\b\b\b\b\bline2\r\n\x1b[7m--More--\x1b[m\r        \rline3\r\n"
	if got := cleanTerminal(in); got != "line1\nline2\nline3\n" {
		t.Fatalf("got %q", got)
	}
	for _, p := range []string{" --More-- ", "---(more 42%)---", "-- More --", "<--- More --->", "Press any key to continue (Q to quit)"} {
		if !pagerRe.MatchString(p) {
			t.Errorf("pager %q not recognised", p)
		}
	}
	for _, p := range []string{"R1#", "user@mx01> ", "FGT60E # ", "RP/0/RSP0/CPU0:xr#", "root@(bigip1)(cfg-sync Standalone)(Active)(/Common)(tmos)# ", "[Expert@gw:0]# "} {
		if !promptRe.MatchString(p) {
			t.Errorf("prompt %q not recognised", p)
		}
	}
}

func TestHostCommandQuoting(t *testing.T) {
	cmd := `grep -E '^x' /etc/a | awk '{print $1}'`
	if got := remotetest.Unquote(strings.TrimPrefix(HostCommand(cmd, ""), "sh -c ")); got != cmd {
		t.Fatalf("round trip: %q", got)
	}
	if !strings.HasPrefix(HostCommand("Get-Item x", "powershell"), "powershell -NoProfile -NonInteractive -EncodedCommand ") {
		t.Fatal("powershell")
	}
}

func TestJumpHost(t *testing.T) {
	target := remotetest.Start(t, remotetest.Options{Exec: linuxHost().Handle})
	bastion := remotetest.Start(t, remotetest.Options{})
	cfg := testConfig(t, target)
	bastion.Client = target.Client
	a, _ := os.ReadFile(cfg.KnownHosts)
	b, _ := os.ReadFile(bastion.WriteKnownHosts(t, nil))
	cfg.KnownHosts = filepath.Join(t.TempDir(), "known_hosts")
	os.WriteFile(cfg.KnownHosts, append(a, b...), 0o600)
	jump := endpoint(bastion)
	cfg.Jump = &jump
	c, err := Dial(context.Background(), endpoint(target), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	h, _ := DetectHost(context.Background(), c)
	if h.Family != "ubuntu-22.04" || bastion.Forwarded() != 1 {
		t.Fatalf("via jump: %+v, forwarded %d", h, bastion.Forwarded())
	}
	// The bastion's key is verified too.
	cfg.KnownHosts = target.WriteKnownHosts(t, nil)
	if _, err := Dial(context.Background(), endpoint(target), cfg); !errors.Is(err, ErrUnknownHost) {
		t.Fatalf("unverified bastion must fail: %v", err)
	}
}
