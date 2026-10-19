// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// wide are the East Asian Wide and Fullwidth ranges (plus emoji presentation blocks) that a
// terminal draws two cells wide: CJK ideographs, kana, Hangul, fullwidth forms.
var wide = [][2]rune{
	{0x1100, 0x115F}, {0x231A, 0x231B}, {0x2329, 0x232A}, {0x23E9, 0x23EC}, {0x23F0, 0x23F0},
	{0x23F3, 0x23F3}, {0x25FD, 0x25FE}, {0x2614, 0x2615}, {0x2648, 0x2653}, {0x267F, 0x267F},
	{0x2693, 0x2693}, {0x26A1, 0x26A1}, {0x26AA, 0x26AB}, {0x26BD, 0x26BE}, {0x26C4, 0x26C5},
	{0x26CE, 0x26CE}, {0x26D4, 0x26D4}, {0x26EA, 0x26EA}, {0x26F2, 0x26F3}, {0x26F5, 0x26F5},
	{0x26FA, 0x26FA}, {0x26FD, 0x26FD}, {0x2705, 0x2705}, {0x270A, 0x270B}, {0x2728, 0x2728},
	{0x274C, 0x274C}, {0x274E, 0x274E}, {0x2753, 0x2755}, {0x2757, 0x2757}, {0x2795, 0x2797},
	{0x27B0, 0x27B0}, {0x27BF, 0x27BF}, {0x2B1B, 0x2B1C}, {0x2B50, 0x2B50}, {0x2B55, 0x2B55},
	{0x2E80, 0x303E}, {0x3041, 0x33FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF},
	{0xA960, 0xA97F}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF}, {0xFE10, 0xFE19}, {0xFE30, 0xFE6F},
	{0xFF00, 0xFF60}, {0xFFE0, 0xFFE6}, {0x16FE0, 0x16FE4}, {0x17000, 0x18CFF}, {0x1B000, 0x1B2FF},
	{0x1F004, 0x1F004}, {0x1F0CF, 0x1F0CF}, {0x1F18E, 0x1F18E}, {0x1F191, 0x1F19A}, {0x1F200, 0x1F251},
	{0x1F300, 0x1F64F}, {0x1F680, 0x1F6FF}, {0x1F7E0, 0x1F7EB}, {0x1F900, 0x1F9FF}, {0x1FA70, 0x1FAFF},
	{0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

// RuneWidth is the number of terminal cells r occupies: 0 for combining marks and other
// zero-width characters (so decomposed Vietnamese diacritics align), 2 for wide CJK, else 1.
func RuneWidth(r rune) int {
	switch {
	case r == 0, r == 0x200B, r == 0x200C, r == 0x200D, r == 0x2060, r == 0xFEFF,
		r >= 0xFE00 && r <= 0xFE0F, r >= 0xE0100 && r <= 0xE01EF:
		return 0
	case r < 0x20 || r == 0x7F:
		return 0
	case r < 0x300:
		return 1
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case r >= 0x1160 && r <= 0x11FF: // Hangul jamo medial vowels and final consonants
		return 0
	}
	lo, hi := 0, len(wide)-1
	for lo <= hi {
		m := (lo + hi) / 2
		switch {
		case r < wide[m][0]:
			hi = m - 1
		case r > wide[m][1]:
			lo = m + 1
		default:
			return 2
		}
	}
	return 1
}

// escLen returns the length of an ANSI escape sequence (CSI "ESC [ ... final" or OSC
// "ESC ] ... BEL|ESC \") at the start of s, or 0.
func escLen(s string) int {
	if len(s) < 2 || s[0] != 0x1b {
		return 0
	}
	switch s[1] {
	case '[':
		for i := 2; i < len(s); i++ {
			if s[i] >= 0x40 && s[i] <= 0x7e {
				return i + 1
			}
		}
		return len(s)
	case ']':
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	}
	return 2
}

// StripANSI removes escape sequences.
func StripANSI(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if n := escLen(s[i:]); n > 0 {
			i += n
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// Width is the display width of s in cells, ignoring escape sequences.
func Width(s string) int {
	w := 0
	for i := 0; i < len(s); {
		if n := escLen(s[i:]); n > 0 {
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w += RuneWidth(r)
		i += size
	}
	return w
}

// Truncate shortens s to at most w cells, ending in ellipsis when cut. Escape sequences
// are kept (and a reset appended when one was open) so colours never bleed.
func Truncate(s string, w int, ellipsis string) string {
	if Width(s) <= w {
		return s
	}
	ew := Width(ellipsis)
	if w <= ew {
		ellipsis, ew = "", 0
	}
	var b strings.Builder
	cur, styled := 0, false
	for i := 0; i < len(s); {
		if n := escLen(s[i:]); n > 0 {
			b.WriteString(s[i : i+n])
			styled = true
			i += n
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := RuneWidth(r)
		if cur+rw > w-ew {
			break
		}
		b.WriteRune(r)
		cur += rw
		i += size
	}
	b.WriteString(ellipsis)
	if styled {
		b.WriteString(reset)
	}
	return b.String()
}

// PadRight pads s with spaces to w cells (s is returned unchanged when already wider).
func PadRight(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// PadLeft right-aligns s in w cells.
func PadLeft(s string, w int) string {
	if d := w - Width(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}

// Fit truncates or pads s to exactly w cells.
func Fit(s string, w int, ellipsis string) string {
	return PadRight(Truncate(s, w, ellipsis), w)
}

// Wrap breaks plain text into lines of at most w cells, at spaces where possible and
// between any two wide (CJK) characters, which need no spaces.
func Wrap(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		var line strings.Builder
		lw, emitted := 0, false
		flush := func() {
			lines = append(lines, strings.TrimRight(line.String(), " "))
			line.Reset()
			lw, emitted = 0, true
		}
		for _, word := range splitWords(para) {
			ww := Width(word)
			if lw > 0 && lw+ww > w {
				flush()
				if word == " " {
					continue
				}
			}
			for ww > w { // a single word longer than the line: hard-break it
				head := Truncate(word, w, "")
				if head == "" {
					_, n := utf8.DecodeRuneInString(word)
					head = word[:n]
				}
				line.WriteString(head)
				flush()
				word = word[len(head):]
				ww = Width(word)
			}
			if word != "" {
				line.WriteString(word)
				lw += ww
			}
		}
		if lw > 0 || !emitted {
			flush()
		}
	}
	return lines
}

// noBreakBefore are characters that must not begin a wrapped line.
const noBreakBefore = "、。，．・：；？！）」』】〕〉》％,.;:!?)]}…ー"

// JoinLines turns hard-wrapped lines back into one paragraph: lines are joined with a space,
// or with nothing next to a wide (CJK) character. Blank lines separate paragraphs.
func JoinLines(s string) string {
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			switch {
			case l == "" || prev == "":
				b.WriteString("\n")
			default:
				pr, _ := utf8.DecodeLastRuneInString(prev)
				nr, _ := utf8.DecodeRuneInString(l)
				if RuneWidth(pr) != 2 && RuneWidth(nr) != 2 && !strings.ContainsRune(noBreakBefore, nr) {
					b.WriteString(" ")
				}
			}
		}
		b.WriteString(l)
	}
	return b.String()
}

// splitWords splits at spaces (kept as their own tokens) and around wide runes.
func splitWords(s string) []string {
	var out []string
	var cur strings.Builder
	push := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case strings.ContainsRune(noBreakBefore, r) && (cur.Len() > 0 || len(out) > 0):
			// Closing punctuation never starts a line (kinsoku): glue it to what precedes.
			if cur.Len() == 0 && out[len(out)-1] != " " {
				out[len(out)-1] += string(r)
			} else {
				cur.WriteRune(r)
			}
		case r == ' ':
			push()
			out = append(out, " ")
		case RuneWidth(r) == 2:
			push()
			out = append(out, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	push()
	return out
}
