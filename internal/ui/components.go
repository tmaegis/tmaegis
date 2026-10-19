// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"path/filepath"
	"strings"
)

// MaxWidth caps layouts on very wide terminals so lines stay readable.
const MaxWidth = 110

// Printer renders components for one stream. All text it receives is already localized;
// the package itself contains no user-visible words except the brand name.
type Printer struct {
	w io.Writer
	c Caps
	g Glyphs
}

// New returns a Printer writing to w with capabilities c.
func New(w io.Writer, c Caps) *Printer { return &Printer{w: w, c: c, g: c.Glyphs()} }

func (p *Printer) Caps() Caps                      { return p.c }
func (p *Printer) Glyphs() Glyphs                  { return p.g }
func (p *Printer) Writer() io.Writer               { return p.w }
func (p *Printer) Fancy() bool                     { return p.c.Fancy }
func (p *Printer) Print(s string)                  { io.WriteString(p.w, s) }
func (p *Printer) Println(s string)                { io.WriteString(p.w, s+"\n") }
func (p *Printer) Paint(s string, st Style) string { return Paint(s, st, p.c.Color) }

// Width is the usable layout width.
func (p *Printer) Width() int {
	w := p.c.Width
	if w <= 0 {
		w = 80
	}
	if w > MaxWidth {
		w = MaxWidth
	}
	return w
}

// Colour shortcuts.
func (p *Printer) Brand(s string) string   { return p.Paint(s, Style{FG: &Cyan, Bold: true}) }
func (p *Printer) Accent(s string) string  { return p.Paint(s, fg(Cyan)) }
func (p *Printer) Good(s string) string    { return p.Paint(s, fg(Emerald)) }
func (p *Printer) Bad(s string) string     { return p.Paint(s, fg(Red)) }
func (p *Printer) Caution(s string) string { return p.Paint(s, fg(Amber)) }
func (p *Printer) Dim(s string) string     { return p.Paint(s, Style{FG: &Gray}) }
func (p *Printer) Bold(s string) string    { return p.Paint(s, Style{Bold: true}) }

// ---------------------------------------------------------------- banner

// BannerInfo is the text next to the logo.
type BannerInfo struct {
	Name, Version, Tagline, Subtitle string
	// Beta, when set (e.g. "beta"), is shown right after the version.
	Beta string
}

// shieldUnicode is the TMAegis shield (a "T" inside a heater shield) in block characters;
// shieldMask colours each cell: c = cyan outline, e = emerald emblem.
var (
	shieldUnicode = []string{
		"▄▄▄▄▄▄▄▄",
		"█ ▀██▀ █",
		"█  ██  █",
		"▀▄ ██ ▄▀",
		"  ▀▄▄▀  ",
	}
	shieldASCII = []string{
		".------.",
		"| ==== |",
		"|  ||  |",
		" \\ || / ",
		"  `--'  ",
	}
	shieldMask = []string{
		"cccccccc",
		"c eeee c",
		"c  ee  c",
		"cc ee cc",
		"  cccc  ",
	}
)

func (p *Printer) shieldRow(i int) string {
	rows := shieldASCII
	if p.c.Unicode {
		rows = shieldUnicode
	}
	if p.c.Color == NoColor {
		return rows[i]
	}
	var b strings.Builder
	mask := []rune(shieldMask[i])
	for j, r := range []rune(rows[i]) {
		switch mask[j] {
		case 'c':
			b.WriteString(p.Paint(string(r), fg(Cyan)))
		case 'e':
			b.WriteString(p.Paint(string(r), fg(Emerald)))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (b BannerInfo) betaSuffix() string {
	if b.Beta == "" {
		return ""
	}
	return " " + b.Beta
}

// CompactBanner is the one-line header for short terminals and non-TTY output.
func (p *Printer) CompactBanner(b BannerInfo) string {
	sep := " - "
	if p.c.Unicode {
		sep = " · "
	}
	if !p.c.Fancy {
		return fmt.Sprintf("%s v%s%s%s%s", b.Name, b.Version, b.betaSuffix(), sep, b.Tagline)
	}
	line := p.Accent(p.g.Mark) + " " + p.Brand(b.Name) + " " + p.Dim("v"+b.Version+b.betaSuffix()) + p.Dim(sep) + b.Tagline
	return Truncate(line, p.Width()-1, p.g.Ellipsis)
}

// Banner is the logo with name, version and tagline; it falls back to CompactBanner when
// the output is not a terminal, or the terminal is narrow or short.
func (p *Printer) Banner(b BannerInfo) string {
	if !p.c.Fancy || p.Width() < 48 || (p.c.Height > 0 && p.c.Height < 20) {
		return p.CompactBanner(b) + "\n"
	}
	text := []string{
		"",
		p.Brand(b.Name) + "  " + p.Dim("v"+b.Version+b.betaSuffix()),
		b.Tagline,
		p.Dim(b.Subtitle),
		"",
	}
	var out strings.Builder
	for i := range shieldMask {
		line := "  " + p.shieldRow(i) + "   " + text[i]
		out.WriteString(strings.TrimRight(Truncate(line, p.Width()-1, p.g.Ellipsis), " ") + "\n")
	}
	return out.String()
}

// ---------------------------------------------------------------- headings & panels

// SectionWidth caps headings (and cards) so they line up on wide terminals.
const SectionWidth = 78

// Section is a heading followed by a rule.
func (p *Printer) Section(title string) string {
	head := p.Accent(p.g.Pointer) + " " + p.Bold(title) + " "
	rest := min(p.Width()-1, SectionWidth) - Width(head)
	if rest < 3 {
		rest = 3
	}
	return Truncate(head, p.Width()-4, p.g.Ellipsis) + p.Dim(strings.Repeat(p.g.H, rest))
}

// KV is one row of a key/value panel.
type KV struct{ Key, Value string }

// Panel renders aligned key/value rows, indented by two cells.
func (p *Printer) Panel(rows []KV) string {
	kw := 0
	for _, r := range rows {
		if w := Width(r.Key); w > kw {
			kw = w
		}
	}
	var b strings.Builder
	for _, r := range rows {
		valW := p.Width() - 1 - 2 - kw - 2
		if valW < 8 {
			valW = 8
		}
		b.WriteString("  " + p.Dim(PadRight(r.Key, kw)) + "  " + Truncate(r.Value, valW, p.g.Ellipsis) + "\n")
	}
	return b.String()
}

// ---------------------------------------------------------------- tables

// Column describes one table column.
type Column struct {
	Title string
	Right bool // right-align
	Flex  bool // may shrink (with ellipsis) to fit the terminal
	Min   int  // minimum width when shrinking
	Width int  // preferred width (keeps several tables aligned); content may widen it
}

// TableOpts tune a table.
type TableOpts struct {
	Indent   int
	NoteCol  int // notes start under this column
	NoHeader bool
}

// Table renders rows in aligned columns sized to the terminal; notes[i] (optional, may be
// shorter than rows) is printed dimmed under row i.
func (p *Printer) Table(cols []Column, rows [][]string, notes []string, o TableOpts) string {
	const gap = 2
	n := len(cols)
	widths := make([]int, n)
	for i, c := range cols {
		widths[i] = c.Width
		if w := Width(c.Title); !o.NoHeader && w > widths[i] {
			widths[i] = w
		}
	}
	for _, r := range rows {
		for i := 0; i < n && i < len(r); i++ {
			if w := Width(r[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}
	total := o.Indent + gap*(n-1)
	for _, w := range widths {
		total += w
	}
	// shrink takes cells from the widest flexible column first, so one long column does
	// not starve the others.
	shrink := func(over, floor int) int {
		for over > 0 {
			best := -1
			for i := range cols {
				lo := max(cols[i].Min, 4)
				if floor > 0 {
					lo = floor
				}
				if cols[i].Flex && widths[i] > lo && (best < 0 || widths[i] > widths[best]) {
					best = i
				}
			}
			if best < 0 {
				return over
			}
			widths[best]--
			over--
		}
		return 0
	}
	if over := total - (p.Width() - 1); over > 0 {
		if over = shrink(over, 0); over > 0 {
			shrink(over, 4) // still too wide: squeeze flexible columns to the bone
		}
	}
	pad := strings.Repeat(" ", o.Indent)
	cell := func(s string, i int, last bool) string {
		s = Truncate(s, widths[i], p.g.Ellipsis)
		if cols[i].Right {
			return PadLeft(s, widths[i])
		}
		if last {
			return s
		}
		return PadRight(s, widths[i])
	}
	var b strings.Builder
	line := func(cells []string, style func(string) string) {
		parts := make([]string, n)
		for i := 0; i < n; i++ {
			v := ""
			if i < len(cells) {
				v = cells[i]
			}
			parts[i] = cell(v, i, i == n-1)
			if style != nil {
				parts[i] = style(parts[i])
			}
		}
		b.WriteString(strings.TrimRight(pad+strings.Join(parts, strings.Repeat(" ", gap)), " ") + "\n")
	}
	tw := 0
	for _, w := range widths {
		tw += w
	}
	tw += gap * (n - 1)
	tw = min2(tw, p.Width()-1-o.Indent)
	if !o.NoHeader {
		titles := make([]string, n)
		for i, c := range cols {
			titles[i] = c.Title
		}
		line(titles, func(s string) string { return p.Paint(s, Style{FG: &Slate, Bold: true}) })
		b.WriteString(pad + p.Dim(strings.Repeat(p.g.H, tw)) + "\n")
	}
	noteOff := o.Indent
	for i := 0; i < o.NoteCol && i < n; i++ {
		noteOff += widths[i] + gap
	}
	for ri, r := range rows {
		line(r, nil)
		if ri < len(notes) && notes[ri] != "" {
			prefix := strings.Repeat(" ", noteOff) + p.g.Tee + " "
			avail := p.Width() - 1 - Width(prefix)
			if avail < 10 {
				prefix, avail = pad+"  "+p.g.Tee+" ", p.Width()-1-o.Indent-3-Width(p.g.Tee)
			}
			for li, nl := range strings.Split(notes[ri], "\n") {
				if li > 0 {
					prefix = strings.Repeat(" ", Width(prefix))
				}
				nl = strings.Join(strings.Fields(nl), " ")
				if !strings.Contains(nl, "\x1b") {
					nl = p.Dim(Truncate(nl, avail, p.g.Ellipsis))
				} else {
					nl = Truncate(nl, avail, p.g.Ellipsis)
				}
				b.WriteString(p.Dim(prefix) + nl + "\n")
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------- chips, statuses, bars

// ChipWidth is the display width of every severity chip, so columns align.
const ChipWidth = 10

// Chip is a severity badge that carries the level in text as well as colour.
func (p *Printer) Chip(severity string) string {
	label := strings.ToUpper(strings.TrimSpace(severity))
	if label == "" {
		label = "-"
	}
	var bg, fgc RGB
	switch label {
	case "CRITICAL":
		bg, fgc = Rose, White
	case "HIGH":
		bg, fgc = Orange, Black
	case "MEDIUM":
		bg, fgc = Amber, Black
	case "LOW":
		bg, fgc = Slate, Black
	default:
		bg, fgc = Gray, White
	}
	if p.c.Color == NoColor {
		return PadRight("["+label+"]", ChipWidth)
	}
	return Paint(" "+label+" ", Style{FG: &fgc, BG: &bg, Bold: true}, p.c.Color) + strings.Repeat(" ", max(0, ChipWidth-len(label)-2))
}

// Status renders a result status: glyph + word on Unicode terminals, the legacy
// [PASS]/[FAIL]/[SKIP]/[ERR ] badges in ASCII mode (so they stay greppable).
func (p *Printer) Status(status string, manual bool) string {
	s := strings.ToUpper(status)
	if manual && s == "SKIP" {
		s = "MANUAL"
	}
	var glyph, word string
	var c RGB
	switch s {
	case "PASS":
		glyph, word, c = p.g.OK, "PASS", Emerald
	case "FAIL":
		glyph, word, c = p.g.Fail, "FAIL", Red
	case "ERROR", "ERR":
		glyph, word, c = p.g.Warn, "ERROR", Amber
	case "MANUAL":
		glyph, word, c = p.g.Manual, "MANUAL", Cyan
	default:
		glyph, word, c = p.g.Skip, s, Gray
	}
	if !p.c.Unicode {
		badge := map[string]string{"PASS": "[PASS]", "FAIL": "[FAIL]", "ERROR": "[ERR ]", "MANUAL": "[MANUAL]", "SKIP": "[SKIP]"}[word]
		if badge == "" {
			badge = "[" + word + "]"
		}
		return p.Paint(badge, fg(c))
	}
	return p.Paint(glyph+" "+word, fg(c))
}

// Gauge is a horizontal meter, e.g. "████████░░ 82.0%"; its colour follows the score.
func (p *Printer) Gauge(pct float64, width int, label string) string {
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	filled := int(math.Round(pct / 100 * float64(width)))
	c := Red
	switch {
	case pct >= 80:
		c = Emerald
	case pct >= 50:
		c = Amber
	}
	bar := p.Paint(strings.Repeat(p.g.Full, filled), fg(c)) + p.Dim(strings.Repeat(p.g.Empty, width-filled))
	if !p.c.Unicode {
		bar = "[" + bar + "]"
	}
	return bar + " " + p.Paint(label, Style{FG: &c, Bold: true})
}

// Bar is a small solid bar of value/max scaled to width cells (at least one cell when >0).
func (p *Printer) Bar(value, maxValue, width int, c RGB) string {
	if maxValue <= 0 || value <= 0 {
		return ""
	}
	n := int(math.Round(float64(value) / float64(maxValue) * float64(width)))
	if n < 1 {
		n = 1
	}
	return p.Paint(strings.Repeat(p.g.Full, n), fg(c))
}

// ---------------------------------------------------------------- cards, links, lines

// Card frames lines in a rounded box with a title in the top border.
func (p *Printer) Card(title string, lines []string) string {
	w := min(p.Width()-1, SectionWidth)
	inner := w - 4
	border := func(s string) string { return p.Paint(s, fg(Cyan)) }
	t := " " + Truncate(title, inner-2, p.g.Ellipsis) + " "
	topRest := w - 3 - Width(t)
	if topRest < 0 {
		topRest = 0
	}
	var b strings.Builder
	b.WriteString(border(p.g.TL+p.g.H) + p.Brand(t) + border(strings.Repeat(p.g.H, topRest)+p.g.TR) + "\n")
	for _, l := range lines {
		b.WriteString(border(p.g.V) + " " + Fit(l, inner, p.g.Ellipsis) + " " + border(p.g.V) + "\n")
	}
	b.WriteString(border(p.g.BL+strings.Repeat(p.g.H, w-2)+p.g.BR) + "\n")
	return b.String()
}

// Link styles a file path so it looks (and, where supported, is) clickable.
func (p *Printer) Link(path string) string {
	shown := path
	styled := p.Paint(shown, Style{FG: &Cyan, Underline: true})
	if p.c.Hyperlinks {
		abs, err := filepath.Abs(path)
		if err == nil {
			u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
			if !strings.HasPrefix(u.Path, "/") {
				u.Path = "/" + u.Path
			}
			return "\x1b]8;;" + u.String() + "\x1b\\" + styled + "\x1b]8;;\x1b\\"
		}
	}
	return styled
}

// Code highlights `backtick` spans; without colour the backticks are kept.
func (p *Printer) Code(s string) string {
	if p.c.Color == NoColor || !strings.Contains(s, "`") {
		return s
	}
	parts := strings.Split(s, "`")
	for i := 1; i < len(parts); i += 2 {
		parts[i] = p.Paint(parts[i], Style{FG: &Cyan, Bold: true})
	}
	return strings.Join(parts, "")
}

// Hint is a "next step" line.
func (p *Printer) Hint(s string) string {
	return "  " + p.Accent(p.g.Bullet) + " " + p.Code(s)
}

// markers are the legacy prefixes of localized CLI messages.
var markers = []string{"[!!!]", "[*]", "[+]", "[!]", "[?]", "[i]"}

// Line restyles a message that starts with a legacy marker ("[*] ...", "    [+] ...").
// Fancy Unicode output swaps the marker for a coloured glyph; ASCII output keeps the
// marker (coloured when possible); plain output returns msg unchanged.
func (p *Printer) Line(msg string) string {
	if !p.c.Fancy {
		return msg
	}
	body := strings.TrimLeft(msg, " ")
	indent := msg[:len(msg)-len(body)]
	for _, m := range markers {
		if !strings.HasPrefix(body, m) {
			continue
		}
		rest := strings.TrimPrefix(body, m)
		var glyph string
		var c RGB
		switch m {
		case "[*]":
			glyph, c = p.g.Info, Cyan
		case "[+]":
			glyph, c = p.g.OK, Emerald
		case "[!]":
			glyph, c = p.g.Warn, Amber
		case "[!!!]":
			glyph, c = p.g.Warn, Red
		case "[?]":
			glyph, c = "?", Cyan
		case "[i]":
			glyph, c = p.g.Bullet, Gray
		}
		if !p.c.Unicode {
			glyph = m
		}
		if m == "[!!!]" {
			rest = p.Paint(rest, Style{FG: &Red, Bold: true})
		}
		return indent + p.Paint(glyph, Style{FG: &c, Bold: true}) + p.Code(rest)
	}
	return msg
}

// Problem renders an error or warning: the localized message, then an optional fix hint.
func (p *Printer) Problem(isError bool, msg, hint string) string {
	var b strings.Builder
	if !p.c.Fancy {
		b.WriteString(msg + "\n")
		if hint != "" {
			b.WriteString("  " + hint + "\n")
		}
		return b.String()
	}
	glyph, c := p.g.Fail, Red
	if !isError {
		glyph, c = p.g.Warn, Amber
	}
	b.WriteString(p.Paint(glyph, Style{FG: &c, Bold: true}) + " " + p.Paint(msg, fg(c)) + "\n")
	if hint != "" {
		b.WriteString(p.Hint(hint) + "\n")
	}
	return b.String()
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
