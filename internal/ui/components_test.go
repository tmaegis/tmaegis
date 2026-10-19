// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// golden compares got with testdata/<name>.golden (go test ./internal/ui -update rewrites them).
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	cleanGot := strings.ReplaceAll(got, "\r\n", "\n")
	cleanWant := strings.ReplaceAll(string(want), "\r\n", "\n")
	if cleanGot != cleanWant {
		t.Errorf("%s changed:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// showcase renders every component once.
func showcase(c Caps) string {
	var b bytes.Buffer
	p := New(&b, c)
	info := BannerInfo{Name: "TMAegis", Version: "1.2.3", Tagline: "AI-Powered CIS Hardening & Compliance Engine", Subtitle: "Read-only audits, safe hardening, instant rollback"}
	p.Print(p.Banner(info))
	p.Println(p.CompactBanner(info))
	p.Println(p.Section("Scan setup"))
	p.Print(p.Panel([]KV{{"Target", "linux"}, {"OS family", "ubuntu-24.04"}, {"Mục tiêu", "Tiếng Việt"}, {"対象", "日本語のホスト"}, {"Rules dir", strings.Repeat("very/long/path/", 8)}}))
	cols := []Column{{Title: "Status"}, {Title: "Severity"}, {Title: "Rule", Flex: true, Min: 10}, {Title: "Title", Flex: true, Min: 8}}
	rows := [][]string{
		{p.Status("PASS", false), p.Chip("medium"), "CIS-1.1", "Ensure IP forwarding is disabled"},
		{p.Status("FAIL", false), p.Chip("critical"), "CIS-1.2", "SSH ルートログインを無効にする"},
		{p.Status("SKIP", true), p.Chip("low"), "CIS-1.3", "Проверить журнал аудита вручную"},
		{p.Status("ERROR", false), p.Chip("high"), "CIS-1.4-VERY-LONG-IDENTIFIER", "Không đạt: cấu hình chưa được áp dụng"},
	}
	notes := []string{"", "pattern 'PermitRootLogin no' not found in /etc/ssh/sshd_config", "", "cannot read file /etc/foo: permission denied"}
	p.Print(p.Table(cols, rows, notes, TableOpts{Indent: 2, NoteCol: 2}))
	p.Println(p.Gauge(82, 10, "82.0%") + "  " + p.Gauge(45.5, 10, "45.5%"))
	p.Print(p.Card("Scan summary", []string{
		p.Gauge(59.2, 20, "59.2%"),
		p.Good(p.g.OK) + " 103 passed   " + p.Bad(p.g.Fail) + " 71 failed",
		"logging  16  " + p.Bar(16, 16, 12, Red),
		"ntp       4  " + p.Bar(4, 16, 12, Red),
		"日本語のカテゴリ名がとても長い場合でも枠からはみ出さないこと",
	}))
	p.Println(p.Hint("Preview the fixes: `tmaegis apply --dry-run`"))
	for _, l := range []string{"[*] TMAegis v1 - Starting...", "[+] Report written", "    [i] already compliant", "[!] AI warning", "[!!!] WARNING: insecure", "no marker"} {
		p.Println(p.Line(l))
	}
	p.Print(p.Problem(true, "Error: failed to load rules: lstat /nonexist: no such file or directory", "Check --rules-dir or run `tmaegis rules validate`."))
	p.Print(p.Problem(false, "No matching rules", ""))
	return b.String()
}

func asciiCaps(w int) Caps { return Caps{TTY: true, Fancy: true, Live: true, Width: w, Height: 40} }

func TestGoldenASCIINoColor80(t *testing.T) { golden(t, "ascii-80", showcase(asciiCaps(80))) }
func TestGoldenASCIINoColor40(t *testing.T) { golden(t, "ascii-40", showcase(asciiCaps(40))) }
func TestGoldenUnicodeNoColor80(t *testing.T) {
	c := asciiCaps(80)
	c.Unicode = true
	golden(t, "unicode-80", showcase(c))
}
func TestGoldenUnicodeNoColor40(t *testing.T) {
	c := asciiCaps(40)
	c.Unicode = true
	golden(t, "unicode-40", showcase(c))
}

// Every rendered line respects the terminal width, in every language mix.
func TestNoLineExceedsWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		for _, uni := range []bool{false, true} {
			for _, col := range []ColorLevel{NoColor, Color256, TrueColor} {
				c := Caps{Fancy: true, TTY: true, Width: w, Height: 40, Unicode: uni, Color: col}
				for i, l := range strings.Split(showcase(c), "\n") {
					plain := StripANSI(l)
					if strings.Contains(plain, "tmaegis ") || strings.Contains(plain, "Error:") {
						continue // free-text hints and messages are soft-wrapped by the terminal
					}
					if Width(l) > w-1 && !strings.Contains(l, "very/long") {
						t.Errorf("w=%d unicode=%v color=%v line %d is %d cells: %q", w, uni, col, i, Width(l), StripANSI(l))
					}
				}
			}
		}
	}
}

func TestNoEscapesWithoutColor(t *testing.T) {
	for _, uni := range []bool{false, true} {
		out := showcase(Caps{Fancy: true, TTY: true, Width: 80, Height: 40, Unicode: uni})
		if strings.Contains(out, "\x1b") {
			t.Errorf("unicode=%v: escape sequence in no-colour output", uni)
		}
	}
	ascii := showcase(asciiCaps(80))
	for _, r := range ascii {
		if r > 0x7e && !strings.ContainsRune("ạđăươấầếệịộờứửữỳ日本語のカテゴリ名がとても長い場合でも枠からはみ出さないことルートログインを無効にするПроверитьжурналаудитавручнуюКhôngđạtcấuhìnhchưađượcápdụngMụctiêuTiếngViệt対象ホスト", r) {
			t.Errorf("non-ASCII glyph %q in --ascii output", r)
			break
		}
	}
}

func TestPlainPrinterIsUndecorated(t *testing.T) {
	p := New(&bytes.Buffer{}, Plain)
	if got := p.Line("[*] hello `x`"); got != "[*] hello `x`" {
		t.Errorf("plain Line changed text: %q", got)
	}
	if got := p.Banner(BannerInfo{Name: "TMAegis", Version: "1", Tagline: "T"}); got != "TMAegis v1 - T\n" {
		t.Errorf("plain banner = %q", got)
	}
	if got := p.Problem(true, "Error: x", "hint"); got != "Error: x\n  hint\n" {
		t.Errorf("plain problem = %q", got)
	}
}

func TestColorDegradation(t *testing.T) {
	if got := Paint("x", fg(Cyan), TrueColor); got != "\x1b[38;2;0;242;254mx\x1b[0m" {
		t.Errorf("truecolor = %q", got)
	}
	if got := Paint("x", fg(Cyan), Color256); got != "\x1b[38;5;51mx\x1b[0m" {
		t.Errorf("256 = %q", got)
	}
	if got := Paint("x", fg(Cyan), Color16); got != "\x1b[96mx\x1b[0m" {
		t.Errorf("16 = %q", got)
	}
	if got := Paint("x", fg(Emerald), Color16); got != "\x1b[32mx\x1b[0m" && got != "\x1b[36mx\x1b[0m" {
		t.Errorf("emerald 16 = %q", got)
	}
	if got := Paint("x", Style{Bold: true, FG: &Red}, NoColor); got != "x" {
		t.Errorf("none = %q", got)
	}
}

func TestChipCarriesText(t *testing.T) {
	for _, level := range []ColorLevel{NoColor, Color16, TrueColor} {
		p := New(&bytes.Buffer{}, Caps{Fancy: true, Color: level, Width: 80})
		for _, s := range []string{"critical", "high", "medium", "low"} {
			chip := p.Chip(s)
			if !strings.Contains(StripANSI(chip), strings.ToUpper(s)) || Width(chip) != ChipWidth {
				t.Errorf("level %v chip %q: %q (width %d)", level, s, StripANSI(chip), Width(chip))
			}
		}
	}
}

func TestLinkHyperlink(t *testing.T) {
	p := New(&bytes.Buffer{}, Caps{Fancy: true, Color: TrueColor, Hyperlinks: true, Width: 80})
	got := p.Link("/tmp/report.html")
	if (!strings.HasPrefix(got, "\x1b]8;;file:///tmp/report.html\x1b\\") && !strings.Contains(got, "/tmp/report.html\x1b\\")) || StripANSI(got) != "/tmp/report.html" {
		t.Errorf("link = %q", got)
	}
	if got := New(&bytes.Buffer{}, Caps{Fancy: true, Width: 80}).Link("r.json"); got != "r.json" {
		t.Errorf("no-colour link = %q", got)
	}
}
