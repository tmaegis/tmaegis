// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShippedRulesAreValid guards the bundled rule set: every YAML under rules/
// must parse, validate and have a unique ID.
func TestShippedRulesAreValid(t *testing.T) {
	loaded, err := LoadFromDir("../../rules", "all", 2)
	if err != nil {
		t.Fatalf("shipped rules failed to load: %v", err)
	}
	if len(loaded) < 5 {
		t.Fatalf("expected the bundled rules to load, got %d", len(loaded))
	}
}

func TestValidate(t *testing.T) {
	base := func() Rule {
		return Rule{
			ID: "X-1", Title: "t", Target: "linux", Level: 1, Severity: "high",
			Audit: AuditSpec{Type: "command", Command: "true"},
		}
	}
	cases := map[string]func(*Rule){
		"missing title":       func(r *Rule) { r.Title = "" },
		"bad level":           func(r *Rule) { r.Level = 3 },
		"bad severity":        func(r *Rule) { r.Severity = "meh" },
		"bad audit type":      func(r *Rule) { r.Audit.Type = "nope" },
		"audit no command":    func(r *Rule) { r.Audit.Command = "" },
		"bad regex":           func(r *Rule) { r.Audit = AuditSpec{Type: "file_regex", FilePath: "/x", Pattern: "("} },
		"bad remediation":     func(r *Rule) { r.Remediation.Type = "rm_rf" },
		"cloud_check no name": func(r *Rule) { r.Audit = AuditSpec{Type: "cloud_check"} },
		"file_line missing":   func(r *Rule) { r.Remediation = RemediateSpec{Type: "file_line", FilePath: "/x"} },
	}
	if err := (func() error { r := base(); return r.Validate() })(); err != nil {
		t.Fatalf("valid rule rejected: %v", err)
	}
	for name, mutate := range cases {
		r := base()
		mutate(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestLoadFiltersAndRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rule := func(id, target string, level int) string {
		return "id: " + id + "\ntitle: t\ntarget: " + target + "\nlevel: " + string(rune('0'+level)) +
			"\nseverity: low\naudit:\n  type: command\n  command: 'true'\n"
	}
	write("a.yaml", rule("A", "linux", 1))
	write("b.yaml", rule("B", "linux", 2))
	write("c.yaml", rule("C", "aws", 1))

	got, err := LoadFromDir(dir, "linux", 1)
	if err != nil || len(got) != 1 || got[0].ID != "A" {
		t.Fatalf("filter by target/level: got %v, err %v", got, err)
	}
	if all, _ := LoadFromDir(dir, "all", 2); len(all) != 3 || all[0].ID != "A" {
		t.Fatalf("expected 3 sorted rules, got %v", all)
	}

	write("dup.yaml", rule("A", "linux", 1))
	if _, err := LoadFromDir(dir, "all", 2); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
	os.Remove(filepath.Join(dir, "dup.yaml"))

	write("broken.yaml", "id: [unclosed")
	if _, err := LoadFromDir(dir, "all", 2); err == nil {
		t.Fatal("expected YAML parse error")
	}
}

func TestFilterOSFamilyAndTargets(t *testing.T) {
	r := Rule{Target: "Linux", Level: 1, OSFamily: []string{"ubuntu-22.04", "rocky-9"}}
	cases := []struct {
		f    Filter
		want bool
	}{
		{Filter{MaxLevel: 1}, true},
		{Filter{MaxLevel: 1, Targets: []string{"linux", "nginx"}}, true},
		{Filter{MaxLevel: 1, Targets: []string{"nginx"}}, false},
		{Filter{MaxLevel: 1, OSFamily: "ubuntu-22.04"}, true},
		{Filter{MaxLevel: 1, OSFamily: "debian-12"}, false},
		{Filter{MaxLevel: 0}, false},
	}
	for i, c := range cases {
		if got := c.f.matches(r); got != c.want {
			t.Errorf("case %d: got %v want %v", i, got, c.want)
		}
	}
	// Rules without os_family always apply.
	if !(Filter{MaxLevel: 1, OSFamily: "debian-12"}).matches(Rule{Level: 1}) {
		t.Error("rule without os_family must apply everywhere")
	}
}

func TestRuleShell(t *testing.T) {
	cases := map[string]Rule{
		"powershell": {Target: "IIS"},
		"sh":         {Target: "aws"},
		"bash":       {Target: "linux", Audit: AuditSpec{Shell: "bash"}},
	}
	for want, r := range cases {
		if got := r.Shell(); got != want {
			t.Errorf("%s: want %s, got %s", r.Target, want, got)
		}
	}
}
