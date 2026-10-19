// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"errors"
	"testing"
)

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDetectEnv(t *testing.T) {
	size := func(w, h int) func() (int, int, error) { return func() (int, int, error) { return w, h, nil } }
	noSize := func() (int, int, error) { return 0, 0, errors.New("not a terminal") }
	cases := []struct {
		name    string
		env     Env
		opts    Options
		fancy   bool
		live    bool
		color   ColorLevel
		unicode bool
		width   int
	}{
		{"pipe", Env{IsTTY: false, Size: noSize, Getenv: envOf(nil), GOOS: "linux"}, Options{}, false, false, NoColor, false, 80},
		{"pipe with COLUMNS", Env{Getenv: envOf(map[string]string{"COLUMNS": "132"}), GOOS: "linux"}, Options{}, false, false, NoColor, false, 132},
		{"xterm", Env{IsTTY: true, Size: size(120, 40), Getenv: envOf(map[string]string{"TERM": "xterm"}), GOOS: "linux"}, Options{}, true, true, Color16, true, 120},
		{"256", Env{IsTTY: true, Size: size(100, 30), Getenv: envOf(map[string]string{"TERM": "xterm-256color"}), GOOS: "linux"}, Options{}, true, true, Color256, true, 100},
		{"truecolor", Env{IsTTY: true, Size: size(60, 30), Getenv: envOf(map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"}), GOOS: "linux"}, Options{}, true, true, TrueColor, true, 60},
		{"NO_COLOR", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1"}), GOOS: "linux"}, Options{}, true, true, NoColor, true, 80},
		{"TMAEGIS_NO_COLOR", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"TMAEGIS_NO_COLOR": "1"}), GOOS: "linux"}, Options{}, true, true, NoColor, true, 80},
		{"--no-color beats FORCE_COLOR", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"FORCE_COLOR": "3"}), GOOS: "linux"}, Options{NoColor: true}, true, true, NoColor, true, 80},
		{"FORCE_COLOR on a pipe", Env{Getenv: envOf(map[string]string{"FORCE_COLOR": "1", "TERM": "xterm"}), GOOS: "linux"}, Options{}, true, false, Color16, true, 80},
		{"FORCE_COLOR=3", Env{Getenv: envOf(map[string]string{"FORCE_COLOR": "3"}), GOOS: "linux"}, Options{}, true, false, TrueColor, true, 80},
		{"FORCE_COLOR=0", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"FORCE_COLOR": "0", "TERM": "xterm"}), GOOS: "linux"}, Options{}, true, true, NoColor, true, 80},
		{"TERM=dumb", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"TERM": "dumb"}), GOOS: "linux"}, Options{}, true, false, NoColor, false, 80},
		{"--ascii", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"TERM": "xterm"}), GOOS: "linux"}, Options{ASCII: true}, true, true, Color16, false, 80},
		{"TMAEGIS_ASCII", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"TMAEGIS_ASCII": "1"}), GOOS: "linux"}, Options{}, true, true, Color16, false, 80},
		{"LANG=C", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"LANG": "C"}), GOOS: "linux"}, Options{}, true, true, Color16, false, 80},
		{"LC_ALL beats LANG", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "C"}), GOOS: "linux"}, Options{}, true, true, Color16, true, 80},
		{"CI terminal is not live", Env{IsTTY: true, Size: size(80, 24), Getenv: envOf(map[string]string{"CI": "true", "TERM": "xterm"}), GOOS: "linux"}, Options{}, true, false, Color16, true, 80},
		{"tiny terminal clamps", Env{IsTTY: true, Size: size(10, 5), Getenv: envOf(nil), GOOS: "linux"}, Options{}, true, true, Color16, true, 20},
		{"windows with VT", Env{IsTTY: true, Size: size(120, 30), Getenv: envOf(nil), GOOS: "windows", EnableVT: func() bool { return true }}, Options{}, true, true, TrueColor, true, 120},
		{"windows without VT", Env{IsTTY: true, Size: size(80, 25), Getenv: envOf(nil), GOOS: "windows", EnableVT: func() bool { return false }}, Options{}, true, true, NoColor, false, 80},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectEnv(c.env, c.opts)
			if got.Fancy != c.fancy || got.Live != c.live || got.Color != c.color || got.Unicode != c.unicode || got.Width != c.width {
				t.Errorf("got fancy=%v live=%v color=%v unicode=%v width=%d; want %v %v %v %v %d",
					got.Fancy, got.Live, got.Color, got.Unicode, got.Width, c.fancy, c.live, c.color, c.unicode, c.width)
			}
		})
	}
}

func TestHyperlinksOnlyOnKnownTerminals(t *testing.T) {
	base := map[string]string{"TERM": "xterm-256color"}
	if DetectEnv(Env{IsTTY: true, Getenv: envOf(base), GOOS: "linux"}, Options{}).Hyperlinks {
		t.Error("unknown terminal must not get OSC 8 links")
	}
	base["WT_SESSION"] = "x"
	if !DetectEnv(Env{IsTTY: true, Getenv: envOf(base), GOOS: "linux"}, Options{}).Hyperlinks {
		t.Error("Windows Terminal supports OSC 8")
	}
	if DetectEnv(Env{IsTTY: true, Getenv: envOf(base), GOOS: "linux"}, Options{NoColor: true}).Hyperlinks {
		t.Error("--no-color output must contain no escape sequences at all")
	}
}

func TestForNonFileIsPlain(t *testing.T) {
	if c := For(&testWriter{}, Options{}); c.Fancy || c.Color != NoColor || c.Unicode {
		t.Errorf("buffer caps = %+v", c)
	}
}

type testWriter struct{}

func (*testWriter) Write(p []byte) (int, error) { return len(p), nil }
