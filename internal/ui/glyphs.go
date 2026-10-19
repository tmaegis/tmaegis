// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

// Glyphs are the symbols components draw with.
type Glyphs struct {
	OK, Fail, Warn, Info, Skip, Manual string
	Bullet, Arrow, Pointer, Ellipsis   string
	V, H, TL, TR, BL, BR, Tee          string
	Full, Empty                        string
	Spinner                            []string
	Mark                               string // compact logo mark
}

// UnicodeGlyphs need a UTF-8 terminal with a reasonably complete font.
var UnicodeGlyphs = Glyphs{
	OK: "✔", Fail: "✖", Warn: "⚠", Info: "●", Skip: "○", Manual: "◆",
	Bullet: "›", Arrow: "→", Pointer: "▸", Ellipsis: "…",
	V: "│", H: "─", TL: "╭", TR: "╮", BL: "╰", BR: "╯", Tee: "└",
	Full: "█", Empty: "░",
	Spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	Mark:    "◈",
}

// ASCIIGlyphs render on any console.
var ASCIIGlyphs = Glyphs{
	OK: "[OK]", Fail: "[FAIL]", Warn: "[WARN]", Info: "[*]", Skip: "-", Manual: "[?]",
	Bullet: "-", Arrow: "->", Pointer: ">", Ellipsis: "...",
	V: "|", H: "-", TL: "+", TR: "+", BL: "+", BR: "+", Tee: "`-",
	Full: "#", Empty: ".",
	Spinner: []string{"|", "/", "-", "\\"},
	Mark:    "[T]",
}

// Glyphs returns the set for these capabilities.
func (c Caps) Glyphs() Glyphs {
	if c.Unicode {
		return UnicodeGlyphs
	}
	return ASCIIGlyphs
}
