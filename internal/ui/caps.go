// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package ui renders TMAegis' terminal output: capability detection (TTY, colour depth,
// Unicode), display-width aware layout for every supported language, and the components
// the CLI is built from (banner, panels, tables, chips, gauges, progress, cards).
//
// Decoration is only ever produced for a Caps with Fancy set, which Detect grants to
// terminals (or FORCE_COLOR). Callers keep their legacy plain output for everything else,
// so redirected and machine-readable output stays byte-for-byte stable.
package ui

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// ColorLevel is how many colours a terminal can show.
type ColorLevel int

const (
	NoColor   ColorLevel = iota // plain text
	Color16                     // the 16 ANSI colours
	Color256                    // the xterm 256-colour palette
	TrueColor                   // 24-bit RGB
)

func (c ColorLevel) String() string {
	return [...]string{"none", "16", "256", "truecolor"}[c]
}

// Options are the user's explicit choices (command-line flags).
type Options struct {
	NoColor bool // --no-color
	ASCII   bool // --ascii
}

// Caps describes what an output stream can render.
type Caps struct {
	// TTY is true when the stream is an interactive terminal.
	TTY bool
	// Fancy enables the styled layout (tables, cards, glyphs). It is TTY, or FORCE_COLOR.
	// Without it callers print their plain, script-stable output.
	Fancy bool
	// Live allows a single self-updating line (spinner/progress bar).
	Live bool
	// Color is the colour depth; NoColor never emits an escape sequence.
	Color ColorLevel
	// Unicode allows box drawing, blocks and symbols; otherwise ASCII fallbacks are used.
	Unicode bool
	// Hyperlinks allows OSC 8 links (terminals known to support them).
	Hyperlinks bool
	// CI is true on a continuous-integration runner.
	CI bool
	// Width and Height are the terminal size in cells (Width defaults to 80).
	Width, Height int
}

// Plain is the capability set of a pipe, a file or a buffer.
var Plain = Caps{Width: 80, Height: 24}

// Env is everything Detect looks at, so tests can inject it.
type Env struct {
	IsTTY    bool
	Size     func() (w, h int, err error) // nil: unknown
	Getenv   func(string) string
	GOOS     string
	EnableVT func() bool // Windows: turn on virtual-terminal processing; nil means available
}

// Detect inspects f (normally os.Stdout or os.Stderr) and the process environment.
func Detect(f *os.File, o Options) Caps {
	fd := int(f.Fd())
	env := Env{
		IsTTY:  term.IsTerminal(fd),
		Size:   func() (int, int, error) { return term.GetSize(fd) },
		Getenv: os.Getenv,
		GOOS:   runtime.GOOS,
	}
	if runtime.GOOS == "windows" {
		env.EnableVT = func() bool { return enableVT(f) }
	}
	return DetectEnv(env, o)
}

// For returns the capabilities of w: detected for an *os.File, Plain for anything else
// (buffers in tests, files opened for reports).
func For(w io.Writer, o Options) Caps {
	if f, ok := w.(*os.File); ok {
		return Detect(f, o)
	}
	return Plain
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

// DetectEnv applies the detection rules to an injected environment. Precedence for colour:
// --no-color, TMAEGIS_NO_COLOR, NO_COLOR (any value), then FORCE_COLOR (0/false disables,
// 1/2/3 pick 16/256/truecolor, anything else 16), then the terminal itself. Unicode is
// disabled by --ascii, TMAEGIS_ASCII, TERM=dumb, a non-UTF-8 locale, or a Windows console
// without virtual-terminal processing.
func DetectEnv(e Env, o Options) Caps {
	get := e.Getenv
	if get == nil {
		get = func(string) string { return "" }
	}
	c := Caps{TTY: e.IsTTY, Width: 80, Height: 24}
	c.CI = get("CI") != "" && get("CI") != "false" || get("GITHUB_ACTIONS") != "" || get("BUILD_NUMBER") != "" || get("TF_BUILD") != ""
	termName := strings.ToLower(get("TERM"))
	dumb := termName == "dumb"

	vt := true
	if e.GOOS == "windows" && e.IsTTY && e.EnableVT != nil {
		vt = e.EnableVT()
	}

	force := get("FORCE_COLOR")
	forced := force != "" && truthy(force)
	c.Fancy = e.IsTTY || forced

	// Size.
	sized := false
	if e.IsTTY && e.Size != nil {
		if w, h, err := e.Size(); err == nil && w > 0 {
			c.Width, c.Height, sized = w, h, true
		}
	}
	if !sized {
		if n, err := strconv.Atoi(get("COLUMNS")); err == nil && n > 0 {
			c.Width = n
		}
		if n, err := strconv.Atoi(get("LINES")); err == nil && n > 0 {
			c.Height = n
		}
	}

	// Colour.
	switch {
	case o.NoColor, get("TMAEGIS_NO_COLOR") != "" && truthy(get("TMAEGIS_NO_COLOR")), get("NO_COLOR") != "":
		c.Color = NoColor
	case force != "":
		switch strings.TrimSpace(force) {
		case "0", "false", "no", "off":
			c.Color = NoColor
		case "2":
			c.Color = Color256
		case "3":
			c.Color = TrueColor
		default:
			c.Color = levelFromTerm(get, termName, e.GOOS, true)
		}
	case !e.IsTTY || dumb || !vt:
		c.Color = NoColor
	default:
		c.Color = levelFromTerm(get, termName, e.GOOS, false)
	}

	// Glyphs.
	c.Unicode = c.Fancy && !o.ASCII && !(get("TMAEGIS_ASCII") != "" && truthy(get("TMAEGIS_ASCII"))) &&
		!dumb && vt && utf8Locale(get, e.GOOS)

	c.Live = e.IsTTY && !dumb && !c.CI
	c.Hyperlinks = c.Color != NoColor && e.IsTTY && (get("WT_SESSION") != "" || get("KITTY_WINDOW_ID") != "" ||
		get("VTE_VERSION") != "" || get("WEZTERM_PANE") != "" ||
		map[string]bool{"iTerm.app": true, "WezTerm": true, "vscode": true, "ghostty": true}[get("TERM_PROGRAM")])
	if c.Width < 20 {
		c.Width = 20
	}
	return c
}

// levelFromTerm guesses the colour depth of a terminal from its environment.
func levelFromTerm(get func(string) string, termName, goos string, forced bool) ColorLevel {
	ct := strings.ToLower(get("COLORTERM"))
	switch {
	case ct == "truecolor" || ct == "24bit":
		return TrueColor
	case get("WT_SESSION") != "", get("TERM_PROGRAM") == "iTerm.app", get("TERM_PROGRAM") == "WezTerm",
		get("TERM_PROGRAM") == "vscode", get("TERM_PROGRAM") == "ghostty", strings.Contains(termName, "direct"):
		return TrueColor
	case strings.Contains(termName, "256"):
		return Color256
	case goos == "windows" && !forced:
		// Windows 10+ consoles with virtual-terminal processing render 24-bit colour.
		return TrueColor
	}
	return Color16
}

// utf8Locale reports whether the POSIX locale allows UTF-8 output. An unset locale is
// assumed to be UTF-8 (macOS, most containers); "C"/"POSIX" or a legacy charset is not.
func utf8Locale(get func(string) string, goos string) bool {
	if goos == "windows" {
		return true // the console is switched to code page 65001 at start-up
	}
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := get(k)
		if v == "" {
			continue
		}
		l := strings.ToLower(v)
		return strings.Contains(l, "utf-8") || strings.Contains(l, "utf8")
	}
	return true
}
