// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/remote"
	"github.com/tmaegis/tmaegis/internal/remote/remotetest"
)

func writeTestRule(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// remoteFixture starts a fake Linux host and a fake Cisco device that trust one client key,
// writes rules for both, and points the scan globals at them.
func remoteFixture(t *testing.T) (host, device *remotetest.Server, dev *remotetest.FakeDevice) {
	t.Helper()
	i18n.SetLang("en-US")
	dir := t.TempDir()
	writeTestRule(t, filepath.Join(dir, "linux"), "a.yaml", `id: L.1
title: root login disabled
target: linux
level: 1
severity: high
audit: {type: file_regex, file_path: /etc/ssh/sshd_config, pattern: '(?m)^PermitRootLogin no'}
`)
	writeTestRule(t, filepath.Join(dir, "linux"), "b.yaml", `id: L.2
title: rhel only
target: linux
os_family: [rhel-9]
level: 1
severity: high
audit: {type: command, command: "true"}
`)
	writeTestRule(t, filepath.Join(dir, "linux"), "c.yaml", `id: L.3
title: forwarding off
target: linux
level: 1
severity: medium
audit: {type: command, command: "sysctl -n net.ipv4.ip_forward", expected_output: "0"}
`)
	writeTestRule(t, filepath.Join(dir, "network", "cisco-ios"), "a.yaml", `id: C.1
title: aaa new-model
target: cisco-ios
level: 1
severity: high
audit: {type: config_regex, pattern: '(?m)^aaa new-model\s*$'}
`)
	writeTestRule(t, filepath.Join(dir, "network", "cisco-ios"), "b.yaml", `id: C.2
title: ssh v2
target: cisco-ios
level: 1
severity: high
audit: {type: config_regex, pattern: '(?m)^ip ssh version 2'}
`)
	fh := &remotetest.FakeHost{
		Files: map[string]string{
			"/etc/os-release":      "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
			"/etc/ssh/sshd_config": "PermitRootLogin no\n",
		},
		Commands: map[string]string{"sysctl -n net.ipv4.ip_forward": "1\n"},
	}
	host = remotetest.Start(t, remotetest.Options{Exec: fh.Handle})
	dev = &remotetest.FakeDevice{Prompt: "core1#", PageLines: 3, Commands: map[string]string{
		"show running-config": "hostname core1\n!\naaa new-model\nservice password-encryption\nline vty 0 4\n transport input ssh\nend\n"}}
	device = remotetest.Start(t, remotetest.Options{Device: dev})
	key := host.WriteClientKey(t, "")
	device.Client = host.Client
	known := filepath.Join(t.TempDir(), "known_hosts")
	a, _ := os.ReadFile(host.WriteKnownHosts(t, nil))
	b, _ := os.ReadFile(device.WriteKnownHosts(t, nil))
	if err := os.WriteFile(known, append(a, b...), 0o600); err != nil {
		t.Fatal(err)
	}

	saved := []any{rulesDir, scanTarget, scanFormat, scanOutput, scanLevel, scanQuiet, sshDest, sshInventory, sshCommands, sshParallel, remoteConfig}
	t.Cleanup(func() {
		rulesDir, scanTarget, scanFormat, scanOutput = saved[0].(string), saved[1].(string), saved[2].(string), saved[3].(string)
		scanLevel, scanQuiet, sshDest, sshInventory = saved[4].(int), saved[5].(bool), saved[6].(string), saved[7].(string)
		sshCommands, sshParallel = saved[8].([]string), saved[9].(int)
		remoteConfig = saved[10].(func() remote.Config)
	})
	rulesDir, scanTarget, scanFormat, scanOutput, scanLevel, scanQuiet = dir, "all", "json", "", 1, true
	sshDest, sshInventory, sshCommands, sshParallel = "", "", nil, 1
	remoteConfig = func() remote.Config {
		return remote.Config{KeyFile: key, KnownHosts: known, ConnectTimeout: 3 * time.Second,
			Prompt:        func(string) (string, error) { return "", remote.ErrNoTerminal },
			Getenv:        func(string) string { return "" },
			NoDefaultKeys: true}
	}
	return host, device, dev
}

type jsonReport struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Results  []struct {
		Rule   struct{ ID string } `json:"rule"`
		Status string              `json:"status"`
	} `json:"results"`
}

func readReport(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r jsonReport
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, b)
	}
	got := map[string]string{"_host": r.Hostname, "_os": r.OS}
	for _, res := range r.Results {
		got[res.Rule.ID] = res.Status
	}
	return got
}

func TestRemoteScanSingleHost(t *testing.T) {
	host, _, _ := remoteFixture(t)
	sshDest = fmt.Sprintf("auditor@%s:%d", host.Host, host.Port)
	scanOutput = filepath.Join(t.TempDir(), "r.json")
	if err := runRemoteScan(scanCmd); err != nil {
		t.Fatal(err)
	}
	got := readReport(t, scanOutput)
	want := map[string]string{"_host": "web01", "_os": "linux", "L.1": "PASS", "L.3": "FAIL"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (report %v)", k, got[k], v, got)
		}
	}
	if _, ok := got["L.2"]; ok {
		t.Error("the remote os-release (ubuntu-24.04) must exclude the rhel-9 rule")
	}
}

func TestRemoteScanInventory(t *testing.T) {
	host, device, dev := remoteFixture(t)
	inv := filepath.Join(t.TempDir(), "hosts.txt")
	os.WriteFile(inv, []byte(fmt.Sprintf("# lab\nauditor@%s:%d linux\nauditor@%s:%d cisco-ios  # core\nauditor@127.0.0.1:1 linux\n",
		host.Host, host.Port, device.Host, device.Port)), 0o600)
	sshInventory = inv
	sshParallel = 2
	if err := runRemoteScan(scanCmd); err == nil {
		t.Fatal("json without an output directory must be refused")
	}
	scanOutput = filepath.Join(t.TempDir(), "reports")
	err := runRemoteScan(scanCmd)
	if err == nil || !strings.Contains(err.Error(), "1 of 3 hosts") {
		t.Fatalf("the unreachable host must be reported: %v", err)
	}
	h := readReport(t, filepath.Join(scanOutput, fmt.Sprintf("127.0.0.1_%d.json", host.Port)))
	if h["L.1"] != "PASS" || h["_host"] != "web01" {
		t.Errorf("host report %v", h)
	}
	d := readReport(t, filepath.Join(scanOutput, fmt.Sprintf("127.0.0.1_%d.json", device.Port)))
	if d["C.1"] != "PASS" || d["C.2"] != "FAIL" || d["_os"] != "cisco-ios" || len(d) != 4 {
		t.Errorf("device report %v", d)
	}
	for _, r := range dev.Received() {
		if r != "terminal length 0" && r != "show running-config" && r != "exit" {
			t.Errorf("unexpected command sent to the device: %q", r)
		}
	}
}

func TestRemoteScanGuards(t *testing.T) {
	_, device, _ := remoteFixture(t)
	sshDest = fmt.Sprintf("auditor@%s:%d", device.Host, device.Port)
	scanTarget = "paloalto"
	if err := runRemoteScan(scanCmd); err == nil || !strings.Contains(err.Error(), "--ssh-command") {
		t.Fatalf("unverified vendor must require --ssh-command: %v", err)
	}
	scanTarget = "cisco-ios"
	sshCommands = []string{"show run", "write memory"}
	if err := runRemoteScan(scanCmd); !errors.Is(err, remote.ErrNotAllowed) {
		t.Fatalf("write command must be rejected: %v", err)
	}
	scanTarget = "linux"
	sshCommands = []string{"show run"}
	if err := runRemoteScan(scanCmd); err == nil {
		t.Fatal("--ssh-command is for devices only")
	}

	if err := applyCmd.Flags().Set("ssh", "root@db1"); err != nil {
		t.Fatal(err)
	}
	defer func() { applyCmd.Flags().Lookup("ssh").Changed = false }()
	if err := applyCmd.PreRunE(applyCmd, nil); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("apply --ssh must be refused: %v", err)
	}
}
