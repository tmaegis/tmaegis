// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/rules"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
}

func audit(t *testing.T, a rules.AuditSpec) RuleResult {
	t.Helper()
	rep := NewRunner().RunAudit(context.Background(), []rules.Rule{{ID: "T", Audit: a}})
	return rep.Results[0]
}

func TestAuditCommandExactLineMatch(t *testing.T) {
	skipOnWindows(t)
	if r := audit(t, rules.AuditSpec{Type: "command", Command: "echo on", ExpectedOutput: "on"}); r.Status != StatusPass {
		t.Fatalf("want PASS, got %v (%s)", r.Status, r.Details)
	}
	// A substring of an error message must not count as a pass.
	if r := audit(t, rules.AuditSpec{Type: "command", Command: "echo 'command not found'", ExpectedOutput: "on"}); r.Status != StatusFail {
		t.Fatalf("substring must not pass, got %v", r.Status)
	}
	// A failing command never passes, even if its output matches.
	if r := audit(t, rules.AuditSpec{Type: "command", Command: "echo on; exit 1", ExpectedOutput: "on"}); r.Status != StatusFail {
		t.Fatalf("non-zero exit must fail, got %v", r.Status)
	}
}

func TestAuditFileRegex(t *testing.T) {
	f := filepath.Join(t.TempDir(), "conf")
	os.WriteFile(f, []byte("# c\nPermitRootLogin no\n"), 0o600)
	if r := audit(t, rules.AuditSpec{Type: "file_regex", FilePath: f, Pattern: `(?m)^PermitRootLogin\s+no\b`}); r.Status != StatusPass {
		t.Fatalf("want PASS, got %v", r.Status)
	}
	if r := audit(t, rules.AuditSpec{Type: "file_regex", FilePath: f, Pattern: `(?m)^PermitRootLogin\s+yes`}); r.Status != StatusFail {
		t.Fatalf("want FAIL, got %v", r.Status)
	}
	if r := audit(t, rules.AuditSpec{Type: "file_regex", FilePath: f, Pattern: `(`}); r.Status != StatusErr {
		t.Fatalf("want ERROR for bad regex, got %v", r.Status)
	}
	if r := audit(t, rules.AuditSpec{Type: "file_regex", FilePath: f + ".missing", Pattern: `x`}); r.Status != StatusFail {
		t.Fatalf("missing file must fail, got %v", r.Status)
	}
}

func TestScoreAndFailedRules(t *testing.T) {
	skipOnWindows(t)
	rs := []rules.Rule{
		{ID: "P", Audit: rules.AuditSpec{Type: "command", Command: "true"}},
		{ID: "F", Audit: rules.AuditSpec{Type: "command", Command: "false"}},
	}
	rep := NewRunner().RunAudit(context.Background(), rs)
	if rep.PassedCount != 1 || rep.FailedCount != 1 || rep.TotalRules != 2 {
		t.Fatalf("bad counts: %+v", rep)
	}
	if f := rep.FailedRules(); len(f) != 1 || f[0].ID != "F" {
		t.Fatalf("FailedRules = %v", f)
	}
}

func TestApplyFileLine(t *testing.T) {
	re := regexp.MustCompile(`(?m)^\s*#?\s*PermitEmptyPasswords\b.*$`)
	cases := []struct{ name, in, want string }{
		{"replace commented", "a\n#PermitEmptyPasswords yes\nb\n", "a\nPermitEmptyPasswords no\nb\n"},
		{"append when absent", "a\nb", "a\nb\nPermitEmptyPasswords no\n"},
		{"idempotent", "PermitEmptyPasswords no\n", "PermitEmptyPasswords no\n"},
		{"collapse duplicates", "PermitEmptyPasswords yes\nPermitEmptyPasswords yes\n", "PermitEmptyPasswords no\n\n"},
	}
	for _, c := range cases {
		if got := ApplyFileLine(c.in, re, "PermitEmptyPasswords no"); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestRemediateFileLine(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "conf")
	os.WriteFile(f, []byte("Opt yes\n"), 0o640)
	spec := rules.RemediateSpec{Type: "file_line", FilePath: f, MatchRegex: `(?m)^Opt\b.*$`, ReplaceLine: "Opt no"}
	ctx := context.Background()

	spec.VerifyCmd = "false"
	if err := remediateFileLine(ctx, spec, false); err == nil {
		t.Fatal("failed validation must return an error")
	}
	if b, _ := os.ReadFile(f); string(b) != "Opt yes\n" {
		t.Fatalf("live file must be untouched after failed validation, got %q", b)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("staging file leaked: %v", entries)
	}

	spec.VerifyCmd = "grep -q 'Opt no' {file}"
	if err := remediateFileLine(ctx, spec, true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "Opt yes\n" {
		t.Fatal("dry-run must not modify the file")
	}

	if err := remediateFileLine(ctx, spec, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f); string(b) != "Opt no\n" {
		t.Fatalf("file not updated, got %q", b)
	}
	if info, _ := os.Stat(f); info.Mode().Perm() != 0o640 {
		t.Fatalf("permissions not preserved: %v", info.Mode())
	}
}

func TestRunRemediationCommandAndIaC(t *testing.T) {
	skipOnWindows(t)
	marker := filepath.Join(t.TempDir(), "done")
	rs := []rules.Rule{
		{ID: "IAC", Remediation: rules.RemediateSpec{Type: "iac_patch", Command: "touch " + marker + ".iac"}},
		{ID: "CMD", Remediation: rules.RemediateSpec{Type: "command", Command: "touch " + marker, VerifyCmd: "test -f " + marker}},
	}
	if err := NewRunner().RunRemediation(context.Background(), rs, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("command remediation did not run")
	}
	if _, err := os.Stat(marker + ".iac"); err == nil {
		t.Fatal("iac_patch must never be executed directly")
	}

	bad := []rules.Rule{{ID: "BAD", Remediation: rules.RemediateSpec{Type: "command", Command: "true", VerifyCmd: "false"}}}
	if err := NewRunner().RunRemediation(context.Background(), bad, false); err == nil {
		t.Fatal("failing verification must error")
	}
}

func TestConcurrentAuditKeepsOrderAndCounts(t *testing.T) {
	skipOnWindows(t)
	var rs []rules.Rule
	for i := 0; i < 12; i++ {
		cmd := "sleep 0.05; echo ok"
		if i%3 == 0 {
			cmd = "sleep 0.05; false"
		}
		rs = append(rs, rules.Rule{ID: string(rune('A' + i)), Audit: rules.AuditSpec{Type: "command", Command: cmd, ExpectedOutput: "ok"}})
	}
	start := time.Now()
	rep := NewRunnerWithConcurrency(6).RunAudit(context.Background(), rs)
	if time.Since(start) > 400*time.Millisecond {
		t.Errorf("audits did not run in parallel: %v", time.Since(start))
	}
	if rep.PassedCount != 8 || rep.FailedCount != 4 {
		t.Fatalf("counts: %+v", rep)
	}
	for i, r := range rep.Results {
		if r.Rule.ID != rs[i].ID {
			t.Fatalf("result order changed at %d", i)
		}
	}
}

func TestCloudCheckUnknownIsError(t *testing.T) {
	r := audit(t, rules.AuditSpec{Type: "cloud_check", Check: "aws.does.not_exist"})
	if r.Status != StatusErr || !strings.Contains(r.Details, "unknown cloud check") || !strings.Contains(r.Details, "aws.s3.account_public_access_block") {
		t.Fatalf("got %v %q", r.Status, r.Details)
	}
}

func TestIaCSuggestionNeverExecutes(t *testing.T) {
	skipOnWindows(t)
	marker := filepath.Join(t.TempDir(), "ran")
	r := rules.Rule{ID: "X", Audit: rules.AuditSpec{Type: "command", Command: "false"},
		Remediation: rules.RemediateSpec{Type: "iac_patch", Command: "touch " + marker}}
	if err := NewRunner().RunRemediation(context.Background(), []rules.Rule{r}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("iac_patch command must never run")
	}
}

func TestRequiresFileSkipsAndScoreIgnoresSkips(t *testing.T) {
	skipOnWindows(t)
	present := filepath.Join(t.TempDir(), "kube-apiserver.yaml")
	os.WriteFile(present, []byte("- --profiling=false\n"), 0o600)
	rs := []rules.Rule{
		{ID: "WORKER-NA", Audit: rules.AuditSpec{Type: "command", Command: "false", RequiresFile: []string{present + ".missing"}}},
		{ID: "APPLIES", Audit: rules.AuditSpec{Type: "file_regex", FilePath: present, Pattern: "--profiling=false", RequiresFile: []string{present + ".missing", present}}},
		{ID: "FAILS", Audit: rules.AuditSpec{Type: "command", Command: "false"}},
	}
	rep := NewRunner().RunAudit(context.Background(), rs)
	if rep.Results[0].Status != StatusSkip || !strings.Contains(rep.Results[0].Details, "not applicable") {
		t.Fatalf("missing component must SKIP: %+v", rep.Results[0])
	}
	if rep.Results[1].Status != StatusPass {
		t.Fatalf("any listed path present means the rule applies: %+v", rep.Results[1])
	}
	if got := rep.Score(); got != 50 {
		t.Fatalf("score must ignore skipped rules: got %.1f, want 50", got)
	}
	if (&ScanReport{}).Score() != 0 {
		t.Fatal("empty report score")
	}
}

func TestApplyFileLineAtTopForFirstMatchWinsConfigs(t *testing.T) {
	re := regexp.MustCompile(`(?m)^\s*#?\s*PermitEmptyPasswords\b.*$`)
	in := "Include /etc/ssh/sshd_config.d/*.conf\nMatch User backup\n  ForceCommand internal-sftp\n"
	got := ApplyFileLineAt(in, re, "PermitEmptyPasswords no", true)
	if got != "PermitEmptyPasswords no\n"+in {
		t.Fatalf("must insert before Include/Match, got %q", got)
	}
	// An existing directive is still replaced in place.
	if got := ApplyFileLineAt("Port 22\nPermitEmptyPasswords yes\n", re, "PermitEmptyPasswords no", true); got != "Port 22\nPermitEmptyPasswords no\n" {
		t.Fatalf("replace in place: %q", got)
	}
}

func TestAuditShellBash(t *testing.T) {
	skipOnWindows(t)
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	// Process substitution and ${var//} are bash-only; sh (dash) rejects them.
	cmd := `a=x.y; while read -r l; do echo "$l ${a//./-}"; done < <(echo compliant)`
	if r := audit(t, rules.AuditSpec{Type: "command", Shell: "bash", Command: cmd, ExpectedOutput: "compliant x-y"}); r.Status != StatusPass {
		t.Fatalf("bash audit: %v %s", r.Status, r.Details)
	}
}

func TestAuditDeviceConfigAndManual(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "running.cfg")
	os.WriteFile(cfg, []byte("hostname r1\r\nservice password-encryption\r\nip http server\r\n"), 0o600)
	run := NewRunner()
	rs := []rules.Rule{
		{ID: "ENC", Audit: rules.AuditSpec{Type: "config_regex", Pattern: `(?m)^service password-encryption$`}},
		{ID: "HTTP", Audit: rules.AuditSpec{Type: "config_regex", Pattern: `(?m)^ip http server$`, MustNotMatch: true}},
		{ID: "TEL", Audit: rules.AuditSpec{Type: "config_regex", Pattern: `transport input telnet`, MustNotMatch: true}},
		{ID: "MAN", Audit: rules.AuditSpec{Type: "manual", Command: "Review the MDM profile"}},
	}
	if rep := run.RunAudit(context.Background(), rs); rep.Results[0].Status != StatusSkip {
		t.Fatalf("without --device-config device rules must skip, got %v", rep.Results[0].Status)
	}
	run.DeviceConfig = cfg
	rep := run.RunAudit(context.Background(), rs)
	want := []Status{StatusPass, StatusFail, StatusPass, StatusSkip}
	for i, w := range want {
		if rep.Results[i].Status != w {
			t.Fatalf("%s: want %v, got %v (%s)", rs[i].ID, w, rep.Results[i].Status, rep.Results[i].Details)
		}
	}
	if !strings.Contains(rep.Results[3].Details, "Review the MDM profile") {
		t.Fatalf("manual rules carry their instructions: %q", rep.Results[3].Details)
	}
	if rep.PassedCount != 2 || rep.FailedCount != 1 {
		t.Fatalf("manual/skipped rules are not scored: %d passed %d failed", rep.PassedCount, rep.FailedCount)
	}
}
