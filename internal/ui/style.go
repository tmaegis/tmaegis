// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"fmt"
	"strings"
)

const reset = "\x1b[0m"

// RGB is a 24-bit colour.
type RGB struct{ R, G, B uint8 }

// Brand and semantic colours. Severity uses colour AND text, never colour alone.
var (
	Cyan    = RGB{0x00, 0xF2, 0xFE} // brand primary
	Emerald = RGB{0x10, 0xB9, 0x81} // brand secondary, pass
	Red     = RGB{0xEF, 0x44, 0x44} // fail
	Amber   = RGB{0xF5, 0x9E, 0x0B} // warning, medium
	Orange  = RGB{0xF9, 0x73, 0x16} // high
	Rose    = RGB{0xBE, 0x12, 0x3C} // critical
	Slate   = RGB{0x94, 0xA3, 0xB8} // low, muted
	Gray    = RGB{0x6B, 0x72, 0x80} // dim text, empty bars
	White   = RGB{0xFF, 0xFF, 0xFF}
	Black   = RGB{0x11, 0x18, 0x27}
)

// Style is a set of text attributes.
type Style struct {
	FG, BG               *RGB
	Bold, Dim, Underline bool
}

// ansi16 are the xterm default values of the 16 base colours (index = SGR offset).
var ansi16 = [16]RGB{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0}, {0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func dist(a, b RGB) int {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr*3 + dg*dg*4 + db*db*2
}

// nearest16 maps c to the closest of the 16 ANSI colours.
func nearest16(c RGB) int {
	best, bd := 0, 1<<62
	for i, p := range ansi16 {
		if d := dist(c, p); d < bd {
			best, bd = i, d
		}
	}
	return best
}

// nearest256 maps c into the xterm 6x6x6 cube or the grey ramp.
func nearest256(c RGB) int {
	steps := []int{0, 95, 135, 175, 215, 255}
	idx := func(v uint8) int {
		best, bd := 0, 1<<30
		for i, s := range steps {
			d := int(v) - s
			if d < 0 {
				d = -d
			}
			if d < bd {
				best, bd = i, d
			}
		}
		return best
	}
	ri, gi, bi := idx(c.R), idx(c.G), idx(c.B)
	cube := RGB{uint8(steps[ri]), uint8(steps[gi]), uint8(steps[bi])}
	avg := (int(c.R) + int(c.G) + int(c.B)) / 3
	gi2 := (avg - 8) / 10
	if gi2 < 0 {
		gi2 = 0
	} else if gi2 > 23 {
		gi2 = 23
	}
	gv := uint8(8 + gi2*10)
	if dist(c, RGB{gv, gv, gv}) < dist(c, cube) {
		return 232 + gi2
	}
	return 16 + 36*ri + 6*gi + bi
}

func colorSGR(c RGB, level ColorLevel, bg bool) string {
	switch level {
	case TrueColor:
		if bg {
			return fmt.Sprintf("48;2;%d;%d;%d", c.R, c.G, c.B)
		}
		return fmt.Sprintf("38;2;%d;%d;%d", c.R, c.G, c.B)
	case Color256:
		if bg {
			return fmt.Sprintf("48;5;%d", nearest256(c))
		}
		return fmt.Sprintf("38;5;%d", nearest256(c))
	case Color16:
		n := nearest16(c)
		base := 30
		if bg {
			base = 40
		}
		if n >= 8 {
			return fmt.Sprint(base + 60 + n - 8)
		}
		return fmt.Sprint(base + n)
	}
	return ""
}

// Paint applies st to s at the given colour level; NoColor returns s unchanged.
func Paint(s string, st Style, level ColorLevel) string {
	if level == NoColor || s == "" {
		return s
	}
	var codes []string
	if st.Bold {
		codes = append(codes, "1")
	}
	if st.Dim {
		codes = append(codes, "2")
	}
	if st.Underline {
		codes = append(codes, "4")
	}
	if st.FG != nil {
		codes = append(codes, colorSGR(*st.FG, level, false))
	}
	if st.BG != nil {
		codes = append(codes, colorSGR(*st.BG, level, true))
	}
	if len(codes) == 0 {
		return s
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + s + reset
}

func fg(c RGB) Style { return Style{FG: &c} }
