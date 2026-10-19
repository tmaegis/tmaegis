// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

var updateGolden = flag.Bool("update", false, "rewrite the Markdown golden files")

func mdSample() *engine.ScanReport {
	mk := func(id, title, sev, status, details string, rem rules.RemediateSpec, audit rules.AuditSpec) engine.RuleResult {
		return engine.RuleResult{Rule: rules.Rule{ID: id, Title: title, Severity: sev, Target: "linux", Level: 1,
			Reference: "CIS Linux Benchmark - 1.1", Audit: audit, Remediation: rem}, Status: engine.Status(status), Details: details}
	}
	cmd := rules.RemediateSpec{Type: "command", Command: "chmod 600 /etc/shadow"}
	return &engine.ScanReport{
		Timestamp: time.Date(2026, 3, 4, 5, 6, 0, 0, time.UTC), Hostname: "web-01", OS: "linux", Arch: "amd64",
		TotalRules: 6, PassedCount: 1, FailedCount: 3,
		Results: []engine.RuleResult{
			mk("TMA-LINUX-1.1", "Shadow file permissions", "critical", "FAIL", "mode 644", cmd, rules.AuditSpec{Type: "command"}),
			mk("TMA-LINUX-1.2", "SSH root login", "high", "FAIL", "PermitRootLogin yes", rules.RemediateSpec{Type: "iac_patch", Command: "Set PermitRootLogin no"}, rules.AuditSpec{Type: "command"}),
			mk("TMA-DOCKER-2.1", "Docker socket", "low", "ERROR", "", rules.RemediateSpec{}, rules.AuditSpec{Type: "command"}),
			mk("TMA-LINUX-1.3", "Banner", "low", "PASS", "", cmd, rules.AuditSpec{Type: "command"}),
			mk("TMA-LINUX-1.4", "Review sudoers", "medium", "SKIP", "manual review: check sudoers", rules.RemediateSpec{}, rules.AuditSpec{Type: "manual", Command: "Open /etc/sudoers and review it"}),
			mk("TMA-LINUX-1.5", "No GRUB here", "medium", "SKIP", "not applicable", cmd, rules.AuditSpec{Type: "command"}),
		},
	}
}

func mdRender(t *testing.T, rep *engine.ScanReport, lang string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteWith(&buf, "md", rep, lang, Info{Version: "1.2.3", RulesDir: "rules", Options: []string{"--level 1"}}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestMarkdownGolden(t *testing.T) {
	for _, lang := range []string{"en-US", "zh-Hans"} {
		t.Run(lang, func(t *testing.T) {
			got := mdRender(t, mdSample(), lang)
			path := filepath.Join("testdata", "report."+lang+".md")
			if *updateGolden {
				os.MkdirAll("testdata", 0o755)
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			cleanGot := strings.ReplaceAll(got, "\r\n", "\n")
			cleanWant := strings.ReplaceAll(string(want), "\r\n", "\n")
			if cleanGot != cleanWant {
				t.Fatalf("output differs from %s (run go test ./internal/report -run Golden -update)\n%s", path, got)
			}
		})
	}
}

func TestMarkdownDeterministic(t *testing.T) {
	a, b := mdRender(t, mdSample(), "en-US"), mdRender(t, mdSample(), "en-US")
	if a != b {
		t.Fatal("two renders of the same report differ")
	}
}

func TestMarkdownEscaping(t *testing.T) {
	cases := map[string]string{
		"a|b":                `a\|b`,
		"*bold* _x_":         `\*bold\* \_x\_`,
		"`code`":             "\\`code\\`",
		"<script>x</script>": "&lt;script&gt;x&lt;/script&gt;",
		"# head":             `\# head`,
		"- item":             `\- item`,
		"1. item":            `1\. item`,
		"[l](http://x)":      `\[l\](http://x)`,
		"line1\nline2":       "line1 line2",
		"a‮b":                "ab",
		"\x1b[31mred":        `\[31mred`,
	}
	for in, want := range cases {
		if got := mdEscape(in); got != want {
			t.Errorf("mdEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMarkdownFenceAndCode(t *testing.T) {
	out := mdFence("a\n```\nb\n`````\nc")
	if !strings.HasPrefix(out, "``````\n") || !strings.HasSuffix(out, "\n``````\n") {
		t.Fatalf("fence too short: %q", out)
	}
	if got := mdFence("plain"); got != "```\nplain\n```\n" {
		t.Fatalf("%q", got)
	}
	if got := mdCode("a`b"); got != "``a`b``" {
		t.Fatalf("%q", got)
	}
	if got := mdCode("`x`"); got != "`` `x` ``" {
		t.Fatalf("%q", got)
	}
}

func TestMarkdownHostileContent(t *testing.T) {
	evil := "x | y\n```\n<img src=x onerror=alert(1)>\n````` end\n" + strings.Repeat("A", 20000)
	rep := &engine.ScanReport{Timestamp: time.Unix(0, 0), Hostname: "h|`<b>", OS: "linux", Results: []engine.RuleResult{{
		Rule:   rules.Rule{ID: "T|1", Title: "t | `x` <b>*", Severity: "high", Target: "a|b", Level: 1, Reference: "r_1 | <i>"},
		Status: engine.StatusFail, Details: evil,
	}}}
	out := mdRender(t, rep, "en-US")
	if strings.Contains(out, "<b>") || strings.Contains(out, "<i>") {
		t.Fatalf("raw HTML outside code fence:\n%s", out[:600])
	}
	if len(out) > 12000 {
		t.Fatalf("evidence not capped: %d bytes", len(out))
	}
	// the fence around the evidence must be longer than the longest backtick run inside (5)
	if !strings.Contains(out, "\n``````\nx | y\n") || !strings.Contains(out, "\n``````\n\n") {
		t.Fatalf("unsafe evidence fence:\n%s", out)
	}
	// every table row has the same number of unescaped pipes as its header
	var want int
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, "|") {
			want = 0
			continue
		}
		n := strings.Count(strings.ReplaceAll(l, `\|`, ""), "|")
		if want == 0 {
			want = n
		} else if n != want {
			t.Fatalf("ragged table row %q (%d vs %d)", l, n, want)
		}
	}
}

func TestMarkdownTablesStayValidForCJKAndRTL(t *testing.T) {
	rep := mdSample()
	rep.Results[0].Rule.Title = "影子文件权限 | 密码 ‮ evil"
	rep.Results[1].Rule.ID = "TMA-שלום-1.2"
	for _, lang := range []string{"zh-Hans", "ja", "en-US"} {
		out := mdRender(t, rep, lang)
		if strings.ContainsRune(out, '‮') {
			t.Fatal("bidi override must be stripped")
		}
		rows := 0
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "|") {
				rows++
				if !strings.HasSuffix(l, "|") {
					t.Fatalf("table row not closed: %q", l)
				}
			}
		}
		if rows < 8 {
			t.Fatalf("%s: tables missing (%d rows)", lang, rows)
		}
	}
}

func TestMarkdownStructure(t *testing.T) {
	out := mdRender(t, mdSample(), "en-US")
	for _, want := range []string{"# Compliance Report", "- **Hostname:** web-01", "## Summary", "| Critical |", "## By benchmark",
		"### Critical (1)", "#### TMA-LINUX-1.1", "## Manual review", "## Not applicable on this host", "TMAegis(TM)", "Mozilla Public License"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "<details") {
		t.Error("<details> is not portable")
	}
	empty := mdRender(t, &engine.ScanReport{Timestamp: time.Unix(0, 0)}, "en-US")
	if !strings.Contains(empty, "No controls were evaluated") {
		t.Errorf("empty report: %s", empty)
	}
}

// Every registered format must render through Write, WriteLang and WriteWith.
func TestEveryRegisteredFormatWrites(t *testing.T) {
	for _, f := range Formats {
		for _, lang := range []string{"", "en-US", "zh-Hans"} {
			var buf bytes.Buffer
			if err := WriteLang(&buf, f, mdSample(), lang); err != nil || buf.Len() == 0 {
				t.Errorf("format %s lang %q: err=%v len=%d", f, lang, err, buf.Len())
			}
		}
		var buf bytes.Buffer
		if err := Write(&buf, strings.ToUpper(f), evilReport()); err != nil || buf.Len() == 0 {
			t.Errorf("format %s (upper case): %v", f, err)
		}
	}
}
