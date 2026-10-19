// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestWidth(t *testing.T) {
	cases := map[string]int{
		"":                                0,
		"abc":                             3,
		"Tiếng Việt":                      10, // precomposed
		"Tiếng Việt":                  10, // decomposed: combining marks are zero-width
		"Русский":                         7,
		"日本語":                             6,
		"简体中文 · Beta":                     15,
		"한국어":                             6,
		"ｆｕｌｌ":                            8,
		"\x1b[1;38;2;0;242;254mTM\x1b[0m": 2,
		"\x1b]8;;file:///x\x1b\\link\x1b]8;;\x1b\\": 4,
		"✔ PASS": 6,
		"a‍b":    2,
	}
	for s, want := range cases {
		if got := Width(s); got != want {
			t.Errorf("Width(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestTruncateAndPad(t *testing.T) {
	if got := Truncate("日本語のテキスト", 7, "…"); got != "日本語…" || Width(got) != 7 {
		t.Errorf("CJK truncate = %q (%d)", got, Width(got))
	}
	// A wide rune never straddles the limit.
	if got := Truncate("日本語", 5, ""); got != "日本" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("Проверка доступа", 10, "..."); got != "Проверк..." {
		t.Errorf("Cyrillic truncate = %q", got)
	}
	colored := "\x1b[31mredtext\x1b[0m"
	got := Truncate(colored, 4, "~")
	if StripANSI(got) != "red~" || !strings.HasSuffix(got, reset) {
		t.Errorf("styled truncate = %q", got)
	}
	if got := Truncate("short", 10, "…"); got != "short" {
		t.Errorf("no-op truncate = %q", got)
	}
	if got := PadRight("中文", 6); got != "中文  " {
		t.Errorf("PadRight = %q", got)
	}
	if got := PadLeft("Đạt", 5); got != "  Đạt" {
		t.Errorf("PadLeft = %q", got)
	}
	if got := Fit("Không đạt quy tắc", 9, "…"); Width(got) != 9 {
		t.Errorf("Fit width = %d", Width(got))
	}
}

func TestWrap(t *testing.T) {
	got := Wrap("the quick brown fox jumps", 10)
	if want := []string{"the quick", "brown fox", "jumps"}; !reflect.DeepEqual(got, want) {
		t.Errorf("latin wrap = %q", got)
	}
	// CJK breaks between ideographs, never before closing punctuation.
	for _, l := range Wrap("対応言語の一覧を表示します。現在の言語も表示します。", 10) {
		if Width(l) > 10 {
			t.Errorf("line %q is %d cells", l, Width(l))
		}
		if strings.HasPrefix(l, "。") {
			t.Errorf("line starts with closing punctuation: %q", l)
		}
	}
	// A token longer than the line is hard-broken, and a 1-cell line still terminates.
	if got := Wrap("abcdefghij", 4); !reflect.DeepEqual(got, []string{"abcd", "efgh", "ij"}) {
		t.Errorf("hard wrap = %q", got)
	}
	if got := Wrap("日本", 1); len(got) != 2 {
		t.Errorf("1-cell wrap = %q", got)
	}
}

func TestJoinLines(t *testing.T) {
	if got := JoinLines("hello\nworld\n\nnext"); got != "hello world\n\nnext" {
		t.Errorf("latin = %q", got)
	}
	if got := JoinLines("日本語の\nテキスト"); got != "日本語のテキスト" {
		t.Errorf("cjk = %q", got)
	}
}
