// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"embed"
	"encoding/binary"
	"errors"
)

//go:embed assets/fonts/Geist-Regular.ttf assets/fonts/Geist-SemiBold.ttf
var pdfFontFS embed.FS

// ttfFont is the small part of a TrueType font the PDF writer needs: the character map and the
// advance widths. Glyph outlines stay inside the embedded font program.
type ttfFont struct {
	raw        []byte
	unitsPerEm int
	ascent     int
	descent    int
	capHeight  int
	bbox       [4]int
	advances   []uint16
	cmap       map[rune]uint16
}

func u16(b []byte, o int) int { return int(binary.BigEndian.Uint16(b[o:])) }
func u32(b []byte, o int) int { return int(binary.BigEndian.Uint32(b[o:])) }

func parseTTF(b []byte) (*ttfFont, error) {
	if len(b) < 12 || u32(b, 0) != 0x00010000 {
		return nil, errors.New("not a TrueType (glyf) font")
	}
	tables := map[string][]byte{}
	n := u16(b, 4)
	for i := 0; i < n; i++ {
		e := 12 + 16*i
		if e+16 > len(b) {
			return nil, errors.New("truncated table directory")
		}
		off, ln := u32(b, e+8), u32(b, e+12)
		if off+ln > len(b) {
			return nil, errors.New("table out of range")
		}
		tables[string(b[e:e+4])] = b[off : off+ln]
	}
	head, hhea, hmtx, maxp, cm := tables["head"], tables["hhea"], tables["hmtx"], tables["maxp"], tables["cmap"]
	if len(head) < 54 || len(hhea) < 36 || len(maxp) < 6 || cm == nil || hmtx == nil {
		return nil, errors.New("missing font tables")
	}
	f := &ttfFont{raw: b, unitsPerEm: u16(head, 18)}
	s16 := func(o int) int { return int(int16(binary.BigEndian.Uint16(head[o:]))) }
	f.bbox = [4]int{s16(36), s16(38), s16(40), s16(42)}
	f.ascent = int(int16(binary.BigEndian.Uint16(hhea[4:])))
	f.descent = int(int16(binary.BigEndian.Uint16(hhea[6:])))
	f.capHeight = f.ascent * 7 / 10
	if os2 := tables["OS/2"]; len(os2) >= 90 {
		if c := int(int16(binary.BigEndian.Uint16(os2[88:]))); c > 0 {
			f.capHeight = c
		}
	}
	numGlyphs, nh := u16(maxp, 4), u16(hhea, 34)
	if len(hmtx) < nh*4 || nh == 0 {
		return nil, errors.New("bad hmtx")
	}
	f.advances = make([]uint16, numGlyphs)
	for g := 0; g < numGlyphs; g++ {
		i := g
		if i >= nh {
			i = nh - 1
		}
		f.advances[g] = binary.BigEndian.Uint16(hmtx[i*4:])
	}
	f.cmap = map[rune]uint16{}
	nt := u16(cm, 2)
	best := -1
	for i := 0; i < nt; i++ {
		pid, eid, off := u16(cm, 4+8*i), u16(cm, 6+8*i), u32(cm, 8+8*i)
		if off+4 > len(cm) {
			continue
		}
		format := u16(cm, off)
		if (pid == 3 && eid == 10 || pid == 0 && eid >= 4) && format == 12 {
			best = off
			break
		}
		if (pid == 3 && eid == 1 || pid == 0) && format == 4 && best < 0 {
			best = off
		}
	}
	if best < 0 {
		return nil, errors.New("no usable cmap")
	}
	switch u16(cm, best) {
	case 12:
		ng := u32(cm, best+12)
		for i := 0; i < ng && best+16+12*i+12 <= len(cm); i++ {
			o := best + 16 + 12*i
			start, end, gid := u32(cm, o), u32(cm, o+4), u32(cm, o+8)
			for c := start; c <= end && c-start < 0x10000; c++ {
				f.cmap[rune(c)] = uint16(gid + c - start)
			}
		}
	case 4:
		segX2 := u16(cm, best+6)
		ends, starts := best+14, best+16+segX2
		deltas, ranges := starts+segX2, starts+2*segX2
		for i := 0; i < segX2/2; i++ {
			end, start := u16(cm, ends+2*i), u16(cm, starts+2*i)
			delta, ro := u16(cm, deltas+2*i), u16(cm, ranges+2*i)
			for c := start; c <= end && c != 0xFFFF; c++ {
				var g int
				if ro == 0 {
					g = (c + delta) & 0xFFFF
				} else {
					o := ranges + 2*i + ro + 2*(c-start)
					if o+2 > len(cm) {
						continue
					}
					if g = u16(cm, o); g != 0 {
						g = (g + delta) & 0xFFFF
					}
				}
				if g != 0 {
					f.cmap[rune(c)] = uint16(g)
				}
			}
		}
	}
	return f, nil
}

// has reports whether the font has a glyph for r.
func (f *ttfFont) has(r rune) bool { _, ok := f.cmap[r]; return ok }

// glyph returns the glyph for r, or the one for '?' when the font lacks it.
func (f *ttfFont) glyph(r rune) uint16 {
	if g, ok := f.cmap[r]; ok {
		return g
	}
	return f.cmap['?']
}

// width is the advance of r in 1/1000 em.
func (f *ttfFont) width(r rune) int {
	return int(f.advances[f.glyph(r)]) * 1000 / f.unitsPerEm
}

func loadPDFFont(name string) (*ttfFont, error) {
	b, err := pdfFontFS.ReadFile("assets/fonts/" + name)
	if err != nil {
		return nil, err
	}
	return parseTTF(b)
}
