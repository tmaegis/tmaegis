// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/ui"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

func goldenFile(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./cmd -update)", err)
	}
	cleanGot := strings.ReplaceAll(got, "\r\n", "\n")
	cleanWant := strings.ReplaceAll(string(want), "\r\n", "\n")
	if cleanGot != cleanWant {
		t.Errorf("%s changed:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func sampleReport() *engine.ScanReport {
	rule := func(id, sev, title string, tags ...string) rules.Rule {
		return rules.Rule{ID: id, Severity: sev, Title: title, Target: "cisco-ios", Tags: append([]string{"cisco-ios", "network"}, tags...)}
	}
	manual := rule("TMA-CISCO-IOS-9.9", "low", "Review the banner text with the legal team")
	manual.Audit.Type = "manual"
	r := &engine.ScanReport{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Hostname: "edge-01", OS: "linux", Arch: "amd64",
		Results: []engine.RuleResult{
			{Rule: rule("TMA-CISCO-IOS-1.1.1", "high", "Enable 'aaa new-model'", "aaa"), Status: engine.StatusFail, Details: "pattern '(?m)^aaa new-model\\s*$' not found in running-config.txt"},
			{Rule: rule("TMA-CISCO-IOS-1.5.7", "medium", "Set 'snmp-server host' when using SNMP", "snmp"), Status: engine.StatusPass},
			{Rule: rule("TMA-CISCO-IOS-2.2.4", "medium", "Set IP address for 'logging host'", "logging"), Status: engine.StatusFail, Details: "pattern not found"},
			{Rule: rule("TMA-CISCO-IOS-2.2.7", "critical", "Set 'logging source interface'", "logging"), Status: engine.StatusErr, Details: "cannot read device config"},
			{Rule: rule("TMA-CISCO-IOS-1.3.1", "low", "Set the 'banner-text' for 'banner exec'"), Status: engine.StatusSkip, Details: "not applicable"},
			{Rule: manual, Status: engine.StatusSkip, Details: "manual review: compare with policy"},
		},
	}
	r.TotalRules = len(r.Results)
	r.PassedCount, r.FailedCount = 1, 3
	return r
}

func renderStyled(lang string, width int, unicode bool) string {
	i18n.SetLang(lang)
	defer i18n.SetLang("en-US")
	var b bytes.Buffer
	p := ui.New(&b, ui.Caps{TTY: true, Fancy: true, Width: width, Height: 40, Unicode: unicode})
	r := sampleReport()
	o := summaryOptions{duration: 1500 * time.Millisecond, format: "text", flags: "--target cisco-ios"}
	writeResultsTable(p, r)
	p.Print(summaryCard(p, r, o))
	o.reports = []string{"reports/edge-01.json"}
	p.Print(nextSteps(p, r, o))
	return b.String()
}

func TestGoldenScanASCII80(t *testing.T) {
	goldenFile(t, "scan-ascii-80", renderStyled("en-US", 80, false))
}
func TestGoldenScanASCII40(t *testing.T) {
	goldenFile(t, "scan-ascii-40", renderStyled("en-US", 40, false))
}
func TestGoldenScanUnicode80(t *testing.T) {
	goldenFile(t, "scan-unicode-80", renderStyled("en-US", 80, true))
}
func TestGoldenScanJapanese60(t *testing.T) {
	goldenFile(t, "scan-ja-60", renderStyled("ja", 60, true))
}
func TestGoldenScanVietnamese40(t *testing.T) {
	goldenFile(t, "scan-vi-40", renderStyled("vi", 40, false))
}

func TestStyledScanFitsTheTerminal(t *testing.T) {
	for _, lang := range []string{"en-US", "vi", "ja", "ru", "zh-Hans"} {
		for _, w := range []int{40, 60, 80, 120} {
			for i, l := range strings.Split(renderStyled(lang, w, true), "\n") {
				if strings.Contains(l, "`") {
					continue // copyable command hints are soft-wrapped by the terminal
				}
				if ui.Width(l) > w-1 {
					t.Errorf("%s w=%d line %d is %d cells: %q", lang, w, i, ui.Width(l), l)
				}
			}
		}
	}
}

// Scripts read the text summary from a pipe: it must stay the engine's legacy output.
func TestTextSummaryUnchangedWhenPiped(t *testing.T) {
	r := sampleReport()
	var got, want bytes.Buffer
	finishScan(r, summaryOptions{format: "text"}, &got)
	r.WriteSummary(&want)
	if got.String() != want.String() {
		t.Errorf("piped summary changed:\n%s\nwant:\n%s", got.String(), want.String())
	}
	if bytes.ContainsAny(got.Bytes(), "\x1b✔✖") {
		t.Error("decoration in piped output")
	}
}

func TestPlainBannerAndHelp(t *testing.T) {
	applyLanguage("en-US")
	if b := Banner(); strings.ContainsAny(b, "\x1b🛡") || !strings.HasPrefix(b, "TMAegis v"+Version) {
		t.Errorf("plain banner = %q", b)
	}
	var b bytes.Buffer
	rootCmd.SetOut(&b)
	_ = rootCmd.Help()
	rootCmd.SetOut(nil)
	out := b.String()
	for _, want := range []string{"Audit & harden:", "Inspect:", "Interfaces & extensions:", "Examples:", "tmaegis scan --target linux", "--lang list", "--no-color", "--ascii"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Error("help written to a buffer contains escape sequences")
	}
	if strings.Index(out, "    scan ") > strings.Index(out, "    apply ") {
		t.Error("scan should be listed before apply")
	}
}

func TestOptionsFromArgs(t *testing.T) {
	o := optionsFromArgs([]string{"scan", "--no-color", "--ascii", "--", "--no-color"})
	if !o.NoColor || !o.ASCII {
		t.Errorf("got %+v", o)
	}
	if o := optionsFromArgs([]string{"--", "--ascii"}); o.ASCII {
		t.Error("parsing must stop at --")
	}
}

func TestHintedErrorKeepsMessage(t *testing.T) {
	base := errors.New("failed to load rules: boom")
	err := withHint(base, "fix it")
	if err.Error() != base.Error() || !errors.Is(err, base) {
		t.Errorf("hint changed the error: %v", err)
	}
	if got := hintFor(err, "tmaegis scan"); got != "fix it" {
		t.Errorf("hint = %q", got)
	}
	if got := hintFor(errors.New("unknown flag: --bogus"), "tmaegis scan"); !strings.Contains(got, "tmaegis scan --help") {
		t.Errorf("unknown flag hint = %q", got)
	}
	if hintFor(errors.New("something else"), "tmaegis") != "" {
		t.Error("unexpected hint")
	}
}

func TestCategory(t *testing.T) {
	if got := category(rules.Rule{Target: "cisco-ios", Tags: []string{"cisco-ios", "network", "aaa"}}); got != "aaa" {
		t.Errorf("got %q", got)
	}
	if got := category(rules.Rule{Target: "linux", Tags: []string{"linux"}}); got != "other" {
		t.Errorf("got %q", got)
	}
}
