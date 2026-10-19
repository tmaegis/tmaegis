// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/i18n"
)

// The PDF writer is dependency free. It embeds Geist (TrueType, Identity-H) so Latin, Latin
// Extended (Vietnamese, Serbian Latin, Turkish ...) and Cyrillic render correctly and the text
// stays searchable. It cannot shape CJK, so those languages fall back to English labels.

const (
	pdfW, pdfH   = 595.28, 841.89
	pdfMargin    = 42.0
	pdfFooterTop = pdfH - 44
)

type rgb [3]float64

func rgbHex(c uint32) rgb {
	return rgb{float64(c>>16&255) / 255, float64(c>>8&255) / 255, float64(c&255) / 255}
}

var (
	colDark   = rgbHex(0x0B0F19)
	colPanel  = rgbHex(0x111827)
	colCyan   = rgbHex(0x00F2FE)
	colGreen  = rgbHex(0x10B981)
	colText   = rgbHex(0x0F172A)
	colMuted  = rgbHex(0x5B6B82)
	colLine   = rgbHex(0xE2E8F0)
	colSoft   = rgbHex(0xF1F5F9)
	colPass   = rgbHex(0x047857)
	colPassBg = rgbHex(0xD1FAE5)
	colFail   = rgbHex(0xB91C1C)
	colFailBg = rgbHex(0xFEE2E2)
	colSkip   = rgbHex(0x92400E)
	colSkipBg = rgbHex(0xFEF3C7)
	colWhite  = rgbHex(0xFFFFFF)
	colAmber  = rgbHex(0xD97706)
)

var sevColours = map[string][2]rgb{
	"critical": {rgbHex(0x991B1B), rgbHex(0xFECACA)},
	"high":     {rgbHex(0xC2410C), rgbHex(0xFFEDD5)},
	"medium":   {rgbHex(0xA16207), rgbHex(0xFEF9C3)},
	"low":      {rgbHex(0x1D4ED8), rgbHex(0xDBEAFE)},
	"other":    {colMuted, colSoft},
}

// pdfNoShaping lists the UI languages whose scripts the built-in font cannot draw.
func pdfNoShaping(tag string) bool {
	switch tag {
	case "ja", "ko", "zh-Hans", "zh-Hant":
		return true
	}
	return false
}

type pdfFont struct {
	f    *ttfFont
	name string
	used map[uint16]rune
}

func (pf *pdfFont) encode(s string) (hexStr string, width float64) {
	var b strings.Builder
	units := 0
	for _, r := range s {
		if r < 32 || r == 0x7f {
			r = ' '
		}
		if !pf.f.has(r) {
			r = '?'
		}
		g := pf.f.glyph(r)
		pf.used[g] = r
		fmt.Fprintf(&b, "%04X", g)
		units += int(pf.f.advances[g]) * 1000 / pf.f.unitsPerEm
	}
	return b.String(), float64(units) / 1000
}

func (pf *pdfFont) measure(s string, size float64) float64 {
	units := 0
	for _, r := range s {
		if r < 32 {
			r = ' '
		}
		units += pf.f.width(r)
	}
	return float64(units) * size / 1000
}

type pdfPage struct {
	b bytes.Buffer
}

type pdfDoc struct {
	reg, bold *pdfFont
	pages     []*pdfPage
	cur       *pdfPage
}

func newPDFDoc() (*pdfDoc, error) {
	d := &pdfDoc{}
	for i, name := range []string{"Geist-Regular.ttf", "Geist-SemiBold.ttf"} {
		f, err := loadPDFFont(name)
		if err != nil {
			return nil, err
		}
		pf := &pdfFont{f: f, name: strings.TrimSuffix(name, ".ttf"), used: map[uint16]rune{}}
		if i == 0 {
			d.reg = pf
		} else {
			d.bold = pf
		}
	}
	return d, nil
}

func (d *pdfDoc) newPage() *pdfPage {
	p := &pdfPage{}
	d.pages = append(d.pages, p)
	d.cur = p
	return p
}

func (p *pdfPage) fillColor(c rgb)   { fmt.Fprintf(&p.b, "%.3f %.3f %.3f rg\n", c[0], c[1], c[2]) }
func (p *pdfPage) strokeColor(c rgb) { fmt.Fprintf(&p.b, "%.3f %.3f %.3f RG\n", c[0], c[1], c[2]) }

// rect fills a rectangle; y is measured from the top of the page.
func (p *pdfPage) rect(x, y, w, h float64, c rgb) {
	p.fillColor(c)
	fmt.Fprintf(&p.b, "%.2f %.2f %.2f %.2f re f\n", x, pdfH-y-h, w, h)
}

// text draws one line at baseline y (from the top).
func (p *pdfPage) text(pf *pdfFont, size, x, y float64, c rgb, s string) float64 {
	h, w := pf.encode(s)
	if h == "" {
		return 0
	}
	p.fillColor(c)
	fmt.Fprintf(&p.b, "BT /%s %.2f Tf %.2f %.2f Td <%s> Tj ET\n", map[bool]string{true: "F2", false: "F1"}[strings.Contains(pf.name, "SemiBold")], size, x, pdfH-y, h)
	return w * size
}

func (p *pdfPage) polygon(pts [][2]float64, fill *rgb, stroke *rgb, lw float64) {
	for i, pt := range pts {
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(&p.b, "%.2f %.2f %s\n", pt[0], pdfH-pt[1], op)
	}
	p.b.WriteString("h\n")
	switch {
	case fill != nil && stroke != nil:
		p.fillColor(*fill)
		p.strokeColor(*stroke)
		fmt.Fprintf(&p.b, "%.2f w 1 j B\n", lw)
	case fill != nil:
		p.fillColor(*fill)
		p.b.WriteString("f\n")
	default:
		p.strokeColor(*stroke)
		fmt.Fprintf(&p.b, "%.2f w S\n", lw)
	}
}

// arc strokes part of a circle (angles in turns, 0 = 12 o'clock, clockwise).
func (p *pdfPage) arc(cx, cy, r, from, to, lw float64, c rgb) {
	p.strokeColor(c)
	fmt.Fprintf(&p.b, "%.2f w 1 J 1 j\n", lw)
	steps := int(math.Max(2, math.Ceil((to-from)*96)))
	for i := 0; i <= steps; i++ {
		a := (from + (to-from)*float64(i)/float64(steps)) * 2 * math.Pi
		x, y := cx+r*math.Sin(a), cy-r*math.Cos(a)
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(&p.b, "%.2f %.2f %s\n", x, pdfH-y, op)
	}
	p.b.WriteString("S\n")
}

// logo draws assets/logo-mark.svg as vector paths; (x, y) is its top-left corner, size its edge.
func (p *pdfPage) logo(x, y, size float64) {
	s := size / 120
	pt := func(v ...float64) [][2]float64 {
		out := make([][2]float64, 0, len(v)/2)
		for i := 0; i+1 < len(v); i += 2 {
			out = append(out, [2]float64{x + v[i]*s, y + v[i+1]*s})
		}
		return out
	}
	panel, cyan, green := colPanel, colCyan, colGreen
	p.polygon(pt(60, 6, 108, 25, 108, 68, 60, 114, 12, 68, 12, 25), &panel, &cyan, 5*s)
	p.polygon(pt(24, 31, 37, 36, 60, 70, 60, 86, 37, 56, 37, 77, 24, 64), &green, nil, 0)
	p.polygon(pt(96, 31, 83, 36, 60, 70, 60, 86, 83, 56, 83, 77, 96, 64), &green, nil, 0)
	p.polygon(pt(35, 24, 85, 24, 85, 36, 66, 36, 66, 90, 60, 99, 54, 90, 54, 36, 35, 36), &cyan, nil, 0)
}

// wrapText breaks s into lines no wider than maxW points.
func wrapText(pf *pdfFont, size, maxW float64, s string) []string {
	var lines []string
	for _, para := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		cur := ""
		flush := func() {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
		}
		for _, word := range strings.Fields(para) {
			for pf.measure(word, size) > maxW { // a single token wider than the line: split it
				cut := 0
				for i := range word {
					if i > 0 && pf.measure(word[:i], size) > maxW {
						break
					}
					cut = i
				}
				if cut == 0 {
					cut = len(word)
					if r := []rune(word); len(r) > 1 {
						cut = len(string(r[:1]))
					}
				}
				flush()
				lines = append(lines, word[:cut])
				word = word[cut:]
			}
			try := word
			if cur != "" {
				try = cur + " " + word
			}
			if pf.measure(try, size) > maxW {
				flush()
				cur = word
			} else {
				cur = try
			}
		}
		flush()
	}
	return lines
}

func limitLines(lines []string, n int) []string {
	if len(lines) > n {
		lines = append(append([]string(nil), lines[:n-1]...), lines[n-1]+"...")
	}
	return lines
}

// pill draws a rounded-looking label and returns its width.
func (p *pdfPage) pill(pf *pdfFont, x, y, size float64, fg, bg rgb, s string) float64 {
	w := pf.measure(s, size) + 8
	p.rect(x, y-size-1.5, w, size+5, bg)
	p.text(pf, size, x+4, y, fg, s)
	return w
}

// ---- document layout ----------------------------------------------------------------------

func writePDF(w io.Writer, rep *engine.ScanReport, c *ctx) error {
	d, err := newPDFDoc()
	if err != nil {
		return err
	}
	tr := c.tr
	note := ""
	if pdfNoShaping(c.tag) {
		tr = i18n.New(i18n.Default)
		note = tr.T("report.pdf.cjknote")
	}

	var pass, fail, skip int
	sev := map[string]int{}
	var failed, other []engine.RuleResult
	for _, r := range rep.Results {
		switch r.Status {
		case engine.StatusPass:
			pass++
			other = append(other, r)
		case engine.StatusSkip:
			skip++
			other = append(other, r)
		default:
			fail++
			sev[sevClass(r.Rule.Severity)]++
			failed = append(failed, r)
		}
	}
	rk := func(s string) int {
		if v, ok := sevRank[sevClass(s)]; ok {
			return v
		}
		return 4
	}
	sort.SliceStable(failed, func(i, j int) bool { return rk(failed[i].Rule.Severity) < rk(failed[j].Rule.Severity) })
	sort.SliceStable(other, func(i, j int) bool {
		return other[i].Status == engine.StatusSkip && other[j].Status != engine.StatusSkip
	})

	sc := score(rep)
	evaluated := pass + fail
	when := rep.Timestamp
	if when.IsZero() {
		when = time.Now()
	}

	// ---- cover / summary
	p := d.newPage()
	p.rect(0, 0, pdfW, 124, colDark)
	p.rect(0, 124, pdfW, 3, colCyan)
	p.logo(pdfMargin, 28, 68)
	p.text(d.bold, 24, 124, 64, colWhite, "TMAegis™ "+tr.T("report.pdf.title"))
	host := rep.Hostname
	if host == "" {
		host = "-"
	}
	p.text(d.reg, 10.5, 124, 86, colCyan, fmt.Sprintf("%s  |  %s/%s  |  %s", host, rep.OS, rep.Arch, when.Local().Format("2006-01-02 15:04 MST")))
	p.text(d.reg, 9, 124, 104, rgbHex(0x94A3B8), tr.T("report.app.subtitle"))

	y := 168.0
	p.text(d.bold, 12, pdfMargin, y, colMuted, strings.ToUpper(tr.T("report.pdf.summary")))
	cx, cy, rr := pdfMargin+62, y+78, 50.0
	ring := colAmber
	verdict := tr.T("report.verdict.fair")
	verdictCol := rgbHex(0xB45309)
	switch {
	case sc < 70:
		ring, verdict, verdictCol = colFail, tr.T("report.verdict.poor"), colFail
	case sc >= 90:
		ring, verdict, verdictCol = colPass, tr.T("report.verdict.good"), colPass
	}
	p.arc(cx, cy, rr, 0, 0.9999, 13, colSoft)
	if sc > 0 {
		p.arc(cx, cy, rr, 0, math.Min(sc, 99.99)/100, 13, ring)
	}
	scoreTxt := fmt.Sprintf("%.1f%%", sc)
	sw := d.bold.measure(scoreTxt, 22)
	p.text(d.bold, 22, cx-sw/2, cy+3, colText, scoreTxt)
	lw := d.reg.measure(tr.T("report.pdf.score"), 7.5)
	p.text(d.reg, 7.5, cx-lw/2, cy+16, colMuted, tr.T("report.pdf.score"))
	tx := pdfMargin + 150
	p.text(d.bold, 20, tx, y+54, verdictCol, verdict)
	lead := tr.T("report.lead.none")
	if evaluated > 0 {
		lead = tr.T("report.lead.allpass")
		if fail > 0 {
			lead = tr.N("report.lead.some", fail, "evaluated", evaluated, "urgent", sev["critical"]+sev["high"])
		}
	}
	for i, l := range limitLines(wrapText(d.reg, 11, pdfW-pdfMargin-tx, lead), 4) {
		p.text(d.reg, 11, tx, y+78+float64(i)*15, colText, l)
	}

	// stat boxes
	y = 304
	bw := (pdfW - 2*pdfMargin - 3*10) / 4
	for i, st := range []struct {
		n   int
		key string
		col rgb
	}{{pass, "report.stat.passed", colPass}, {fail, "report.stat.failed", colFail}, {skip, "report.stat.skipped", colSkip}, {len(rep.Results), "report.stat.total", colText}} {
		x := pdfMargin + float64(i)*(bw+10)
		p.rect(x, y, bw, 62, colSoft)
		p.rect(x, y, 3, 62, st.col)
		p.text(d.bold, 24, x+14, y+32, st.col, fmt.Sprint(st.n))
		p.text(d.reg, 9, x+14, y+50, colMuted, tr.T(st.key))
	}

	// severity bars
	y = 396
	p.text(d.bold, 12, pdfMargin, y, colMuted, strings.ToUpper(tr.T("report.pdf.severity")))
	y += 14
	trackX, trackW := pdfMargin+90, pdfW-2*pdfMargin-90-40
	for _, k := range []string{"critical", "high", "medium", "low", "other"} {
		if k == "other" && sev[k] == 0 {
			continue
		}
		p.text(d.reg, 10, pdfMargin, y+11, colText, tr.T("report.sev."+k))
		p.rect(trackX, y+3, trackW, 9, colSoft)
		if fail > 0 && sev[k] > 0 {
			p.rect(trackX, y+3, math.Max(3, trackW*float64(sev[k])/float64(fail)), 9, sevColours[k][0])
		}
		cnt := fmt.Sprint(sev[k])
		p.text(d.reg, 10, pdfW-pdfMargin-d.reg.measure(cnt, 10), y+11, colMuted, cnt)
		y += 20
	}

	// top failures
	y += 18
	p.text(d.bold, 12, pdfMargin, y, colMuted, strings.ToUpper(tr.T("report.pdf.top")))
	y += 10
	limit := pdfH - 120
	if note != "" {
		limit -= 70
	}
	for i := 0; i < len(failed) && i < 8; i++ {
		r := failed[i]
		if y+30 > limit {
			break
		}
		y += 17
		sc2 := sevColours[sevClass(r.Rule.Severity)]
		pw := p.pill(d.bold, pdfMargin, y, 8, sc2[0], sc2[1], tr.T("report.sev."+sevClass(r.Rule.Severity)))
		title := limitLines(wrapText(d.reg, 9.5, pdfW-2*pdfMargin-pw-10, r.Rule.Title), 1)
		if len(title) > 0 {
			p.text(d.reg, 9.5, pdfMargin+pw+8, y, colText, title[0])
		}
	}
	if note != "" {
		ny := pdfH - 150
		lines := wrapText(d.reg, 9, pdfW-2*pdfMargin-24, note)
		p.rect(pdfMargin, ny, pdfW-2*pdfMargin, float64(len(lines))*13+16, colSkipBg)
		for i, l := range lines {
			p.text(d.reg, 9, pdfMargin+12, ny+17+float64(i)*13, colSkip, l)
		}
	}

	// ---- results table
	colStatus, colSev, colID := pdfMargin, pdfMargin+58, pdfMargin+118
	colTitle := pdfMargin + 250
	titleW := pdfW - pdfMargin - colTitle
	idW := colTitle - colID - 8
	header := func() {
		p.text(d.bold, 15, pdfMargin, 66, colText, tr.T("report.pdf.results"))
		p.rect(pdfMargin, 76, pdfW-2*pdfMargin, 18, colPanel)
		p.text(d.bold, 8, colStatus+6, 88, colWhite, tr.T("report.col.status"))
		p.text(d.bold, 8, colSev, 88, colWhite, tr.T("report.col.severity"))
		p.text(d.bold, 8, colID, 88, colWhite, tr.T("report.col.id"))
		p.text(d.bold, 8, colTitle, 88, colWhite, tr.T("report.col.title"))
	}
	p = d.newPage()
	header()
	y = 108
	for _, r := range append(failed, other...) {
		titleLines := limitLines(wrapText(d.reg, 8.5, titleW, r.Rule.Title), 3)
		idLines := limitLines(wrapText(d.reg, 8, idW, r.Rule.ID), 2)
		var detail []string
		if r.Status != engine.StatusPass && r.Details != "" {
			detail = limitLines(wrapText(d.reg, 7.5, pdfW-2*pdfMargin-(colTitle-pdfMargin)-0, clip(strings.ReplaceAll(r.Details, "\n", " "), 260)), 2)
		}
		n := len(titleLines)
		if len(idLines) > n {
			n = len(idLines)
		}
		rowH := float64(n)*11.5 + float64(len(detail))*10 + 8
		if y+rowH > pdfFooterTop-6 {
			p = d.newPage()
			header()
			y = 108
		}
		st, stBg, stLabel := colPass, colPassBg, tr.T("report.status.pass")
		switch r.Status {
		case engine.StatusFail:
			st, stBg, stLabel = colFail, colFailBg, tr.T("report.status.fail")
		case engine.StatusErr:
			st, stBg, stLabel = colFail, colFailBg, tr.T("report.status.error")
		case engine.StatusSkip:
			st, stBg, stLabel = colSkip, colSkipBg, tr.T("report.status.skip")
		}
		p.pill(d.bold, colStatus+2, y+10, 7.5, st, stBg, stLabel)
		sc2 := sevColours[sevClass(r.Rule.Severity)]
		p.text(d.bold, 8, colSev, y+10, sc2[0], tr.T("report.sev."+sevClass(r.Rule.Severity)))
		for i, l := range idLines {
			p.text(d.reg, 8, colID, y+10+float64(i)*11.5, colMuted, l)
		}
		for i, l := range titleLines {
			p.text(d.reg, 8.5, colTitle, y+10+float64(i)*11.5, colText, l)
		}
		for i, l := range detail {
			p.text(d.reg, 7.5, colTitle, y+10+float64(n)*11.5+float64(i)*10, colMuted, l)
		}
		p.rect(pdfMargin, y+rowH-1, pdfW-2*pdfMargin, 0.5, colLine)
		y += rowH
	}

	// ---- footers
	for i, pg := range d.pages {
		pg.rect(pdfMargin, pdfFooterTop, pdfW-2*pdfMargin, 0.5, colLine)
		pageTxt := tr.T("report.pdf.page", "page", i+1, "pages", len(d.pages))
		pg.text(d.reg, 7.5, pdfW-pdfMargin-d.reg.measure(pageTxt, 7.5), pdfFooterTop+14, colMuted, pageTxt)
		for j, l := range limitLines(wrapText(d.reg, 7, pdfW-2*pdfMargin-110, tr.T("report.pdf.footer")), 2) {
			pg.text(d.reg, 7, pdfMargin, pdfFooterTop+13+float64(j)*9, colMuted, l)
		}
	}
	return d.write(w, tr.T("report.pdf.title")+" - "+host)
}

// ---- serialisation ----------------------------------------------------------------------

type pdfObjs struct{ list [][]byte }

func (o *pdfObjs) add(b []byte) int     { o.list = append(o.list, b); return len(o.list) }
func (o *pdfObjs) reserve() int         { o.list = append(o.list, nil); return len(o.list) }
func (o *pdfObjs) set(id int, b []byte) { o.list[id-1] = b }

func stream(dict string, data []byte) []byte {
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(data)
	zw.Close()
	var b bytes.Buffer
	fmt.Fprintf(&b, "<< %s /Filter /FlateDecode /Length %d >>\nstream\n", dict, z.Len())
	b.Write(z.Bytes())
	b.WriteString("\nendstream")
	return b.Bytes()
}

func pdfString(s string) string {
	// UTF-16BE with BOM, hex encoded, so the document title may contain any language
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			fmt.Fprintf(&b, "%04X%04X", 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			continue
		}
		fmt.Fprintf(&b, "%04X", r)
	}
	b.WriteString(">")
	return b.String()
}

func (d *pdfDoc) fontObjects(o *pdfObjs, pf *pdfFont) int {
	f := pf.f
	scale := func(v int) int { return v * 1000 / f.unitsPerEm }
	fileID := o.add(stream(fmt.Sprintf("/Length1 %d", len(f.raw)), f.raw))
	descID := o.add([]byte(fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 32 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
		pf.name, scale(f.bbox[0]), scale(f.bbox[1]), scale(f.bbox[2]), scale(f.bbox[3]), scale(f.ascent), scale(f.descent), scale(f.capHeight), fileID)))
	gids := make([]int, 0, len(pf.used))
	for g := range pf.used {
		gids = append(gids, int(g))
	}
	sort.Ints(gids)
	var w, cm strings.Builder
	w.WriteString("[")
	for _, g := range gids {
		fmt.Fprintf(&w, "%d [%d] ", g, int(f.advances[g])*1000/f.unitsPerEm)
	}
	w.WriteString("]")
	cm.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for i := 0; i < len(gids); i += 100 {
		end := i + 100
		if end > len(gids) {
			end = len(gids)
		}
		fmt.Fprintf(&cm, "%d beginbfchar\n", end-i)
		for _, g := range gids[i:end] {
			r := pf.used[uint16(g)]
			if r > 0xFFFF {
				r = '?'
			}
			fmt.Fprintf(&cm, "<%04X> <%04X>\n", g, r)
		}
		cm.WriteString("endbfchar\n")
	}
	cm.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend")
	tuID := o.add(stream("", []byte(cm.String())))
	cidID := o.add([]byte(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /DW 600 /W %s /CIDToGIDMap /Identity >>",
		pf.name, descID, w.String())))
	return o.add([]byte(fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>", pf.name, cidID, tuID)))
}

func (d *pdfDoc) write(w io.Writer, title string) error {
	o := &pdfObjs{}
	catalog, pagesID := o.reserve(), o.reserve()
	f1 := d.fontObjects(o, d.reg)
	f2 := d.fontObjects(o, d.bold)
	info := o.add([]byte(fmt.Sprintf("<< /Title %s /Producer (TMAegis) /Creator (TMAegis) >>", pdfString(title))))
	kids := make([]string, len(d.pages))
	for i, pg := range d.pages {
		cid := o.add(stream("", pg.b.Bytes()))
		pid := o.add([]byte(fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>", pagesID, pdfW, pdfH, f1, f2, cid)))
		kids[i] = fmt.Sprintf("%d 0 R", pid)
	}
	o.set(pagesID, []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.pages))))
	o.set(catalog, []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesID)))

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(o.list))
	for i, b := range o.list {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(b)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(o.list)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(o.list)+1, catalog, info, xref)
	_, err := w.Write(out.Bytes())
	return err
}
