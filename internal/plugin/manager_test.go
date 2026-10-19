// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverAndExecute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "got.json")
	script := "#!/bin/sh\ncat > " + out + "\n"
	os.WriteFile(filepath.Join(dir, "tmaegis-plugin-echo.sh"), []byte(script), 0o755)
	os.WriteFile(filepath.Join(dir, "thing.wasm"), []byte{0}, 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644)

	m := NewManager(dir)
	list, err := m.Discover()
	if err != nil || len(list) != 2 {
		t.Fatalf("discover: %v %v", list, err)
	}
	if err := m.Execute(context.Background(), "echo", map[string]int{"total_rules": 3}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != `{"total_rules":3}` {
		t.Fatalf("plugin stdin = %q", b)
	}
	if err := m.Execute(context.Background(), "missing", nil); err == nil {
		t.Fatal("unknown plugin must error")
	}
	if l, err := NewManager(filepath.Join(dir, "nope")).Discover(); err != nil || len(l) != 0 {
		t.Fatalf("missing dir should be empty: %v %v", l, err)
	}
}

func TestBundledPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	m := NewManager("../../plugins")
	list, err := m.Discover()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range list {
		have[p.Name] = true
	}
	for _, want := range []string{"notify-webhook", "slack", "telegram", "jira"} {
		if !have[want] {
			t.Errorf("bundled plugin %q not discovered", want)
		}
	}
	// Notifier plugins must fail loudly (not silently) without their configuration.
	t.Setenv("SLACK_WEBHOOK_URL", "")
	if err := m.Execute(context.Background(), "slack", map[string]int{"total_rules": 1}); err == nil {
		t.Error("slack plugin must error without SLACK_WEBHOOK_URL")
	}
	t.Setenv("JIRA_DRY_RUN", "1")
	t.Setenv("JIRA_PROJECT", "SEC")
	if _, err := exec.LookPath("python3"); err == nil {
		report := map[string]interface{}{"results": []map[string]interface{}{
			{"status": "FAIL", "details": "d", "rule": map[string]string{"id": "R1", "title": "t", "severity": "critical"}},
			{"status": "FAIL", "rule": map[string]string{"id": "R2", "severity": "low"}},
			{"status": "PASS", "rule": map[string]string{"id": "R3", "severity": "high"}},
		}}
		if err := m.Execute(context.Background(), "jira", report); err != nil {
			t.Errorf("jira dry run failed: %v", err)
		}
	}
}
