// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// Cell style indexes of styles.xml (cellXfs order).
const (
	sDefault = iota
	sTitle
	sHeader
	sWrap
	sLabel
	sScore
	sInt
	sMuted
	sPct
)

// Differential formats (conditional formatting), in dxfs order.
const (
	dxPass = iota
	dxFail
	dxSkip
	dxCrit
	dxHigh
	dxMed
	dxLow
)

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<numFmts count="2"><numFmt numFmtId="164" formatCode="0.0&quot;%&quot;"/><numFmt numFmtId="165" formatCode="0.0%"/></numFmts>
<fonts count="6">
<font><sz val="11"/><name val="Calibri"/><family val="2"/></font>
<font><b/><sz val="18"/><color rgb="FF0369A1"/><name val="Calibri"/><family val="2"/></font>
<font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Calibri"/><family val="2"/></font>
<font><b/><sz val="11"/><name val="Calibri"/><family val="2"/></font>
<font><b/><sz val="26"/><color rgb="FF0369A1"/><name val="Calibri"/><family val="2"/></font>
<font><i/><sz val="10"/><color rgb="FF5B6B82"/><name val="Calibri"/><family val="2"/></font>
</fonts>
<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF111827"/><bgColor indexed="64"/></patternFill></fill></fills>
<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border>
<border><left/><right/><top/><bottom style="thin"><color rgb="FFE2E8F0"/></bottom><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="9">
<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>
<xf numFmtId="0" fontId="2" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf>
<xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyBorder="1" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf>
<xf numFmtId="0" fontId="3" fillId="0" borderId="0" xfId="0" applyFont="1"/>
<xf numFmtId="164" fontId="4" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1" applyAlignment="1"><alignment horizontal="left"/></xf>
<xf numFmtId="1" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyAlignment="1"><alignment horizontal="left"/></xf>
<xf numFmtId="0" fontId="5" fillId="0" borderId="0" xfId="0" applyFont="1"/>
<xf numFmtId="165" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyAlignment="1"><alignment horizontal="left"/></xf>
</cellXfs>
<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
<dxfs count="7">
<dxf><font><b/><color rgb="FF047857"/></font><fill><patternFill><bgColor rgb="FFD1FAE5"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FFB91C1C"/></font><fill><patternFill><bgColor rgb="FFFEE2E2"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FF92400E"/></font><fill><patternFill><bgColor rgb="FFFEF3C7"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FF991B1B"/></font><fill><patternFill><bgColor rgb="FFFECACA"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FFC2410C"/></font><fill><patternFill><bgColor rgb="FFFFEDD5"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FFA16207"/></font><fill><patternFill><bgColor rgb="FFFEF9C3"/></patternFill></fill></dxf>
<dxf><font><b/><color rgb="FF1D4ED8"/></font><fill><patternFill><bgColor rgb="FFDBEAFE"/></patternFill></fill></dxf>
</dxfs>
</styleSheet>`

// sheetName makes a legal Excel sheet name (max 31 characters, none of []:*?/\).
func sheetName(s string, used map[string]bool) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return ' '
		}
		return r
	}, s)
	r := []rune(strings.TrimSpace(s))
	if len(r) > 31 {
		r = r[:31]
	}
	name := string(r)
	if name == "" {
		name = "Sheet"
	}
	for base, i := name, 2; used[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s %d", string([]rune(base)[:min(len([]rune(base)), 27)]), i)
	}
	used[strings.ToLower(name)] = true
	return name
}

func colName(i int) string { return string(rune('A' + i)) } // up to 26 columns

func xmlText(s string) string {
	var esc bytes.Buffer
	xml.EscapeText(&esc, []byte(s))
	return esc.String()
}

// sheetBuilder writes rows of cells in order.
type sheetBuilder struct {
	b   bytes.Buffer
	row int
}

func (s *sheetBuilder) open(height float64) {
	s.row++
	if height > 0 {
		fmt.Fprintf(&s.b, `<row r="%d" ht="%.1f" customHeight="1">`, s.row, height)
	} else {
		fmt.Fprintf(&s.b, `<row r="%d">`, s.row)
	}
}
func (s *sheetBuilder) close() { s.b.WriteString(`</row>`) }
func (s *sheetBuilder) text(col int, v string, style int) {
	v = clip(csvSafe(v), 30000)
	if v == "" && style == sDefault {
		return
	}
	fmt.Fprintf(&s.b, `<c r="%s%d" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, colName(col), s.row, style, xmlText(v))
}
func (s *sheetBuilder) num(col int, v float64, style int) {
	fmt.Fprintf(&s.b, `<c r="%s%d" s="%d"><v>%s</v></c>`, colName(col), s.row, style, strconv.FormatFloat(v, 'f', -1, 64))
}
func (s *sheetBuilder) blank() { s.row++ }

type xlsxSheet struct {
	name string
	xml  string
}

const sheetNS = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`

func colsXML(widths ...float64) string {
	var b strings.Builder
	b.WriteString("<cols>")
	for i, w := range widths {
		fmt.Fprintf(&b, `<col min="%d" max="%d" width="%.1f" customWidth="1"/>`, i+1, i+1, w)
	}
	b.WriteString("</cols>")
	return b.String()
}

// cfRule is one conditional-formatting rule: cells equal to text get differential format dx.
func cfRules(sqref string, pairs [][2]string, prio *int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<conditionalFormatting sqref="%s">`, sqref)
	for _, p := range pairs {
		*prio++
		fmt.Fprintf(&b, `<cfRule type="cellIs" dxfId="%s" priority="%d" operator="equal"><formula>"%s"</formula></cfRule>`, p[0], *prio, strings.ReplaceAll(xmlText(p[1]), `"`, `""`))
	}
	b.WriteString("</conditionalFormatting>")
	return b.String()
}

func writeXLSX(w io.Writer, rep *engine.ScanReport, c *ctx) error {
	tr := c.tr
	used := map[string]bool{}
	nameSummary := sheetName(tr.T("report.xlsx.sheet.summary"), used)
	nameResults := sheetName(tr.T("report.xlsx.sheet.results"), used)

	statusLabel := func(st engine.Status) string {
		switch st {
		case engine.StatusPass:
			return tr.T("report.status.pass")
		case engine.StatusSkip:
			return tr.T("report.status.skip")
		case engine.StatusErr:
			return tr.T("report.status.error")
		}
		return tr.T("report.status.fail")
	}
	sevLabel := func(s string) string { return tr.T("report.sev." + sevClass(s)) }

	// ---- Results sheet
	var rs sheetBuilder
	hdr := []string{"id", "title", "target", "level", "severity", "status", "details", "remediation"}
	rs.open(30)
	for i, h := range hdr {
		rs.text(i, tr.T("report.xlsx.hdr."+h), sHeader)
	}
	rs.close()
	for _, r := range rep.Results {
		rem, _ := remediationText(r)
		rs.open(0)
		rs.text(0, r.Rule.ID, sWrap)
		rs.text(1, r.Rule.Title, sWrap)
		rs.text(2, r.Rule.Target, sWrap)
		rs.num(3, float64(r.Rule.Level), sWrap)
		rs.text(4, sevLabel(r.Rule.Severity), sWrap)
		rs.text(5, statusLabel(r.Status), sWrap)
		rs.text(6, clip(r.Details, 4000), sWrap)
		rs.text(7, rem, sWrap)
		rs.close()
	}
	last := len(rep.Results) + 1
	prio := 0
	cf := cfRules(fmt.Sprintf("F2:F%d", last), [][2]string{
		{"0", tr.T("report.status.pass")}, {"1", tr.T("report.status.fail")}, {"1", tr.T("report.status.error")}, {"2", tr.T("report.status.skip")}}, &prio) +
		cfRules(fmt.Sprintf("E2:E%d", last), [][2]string{
			{"3", tr.T("report.sev.critical")}, {"4", tr.T("report.sev.high")}, {"5", tr.T("report.sev.medium")}, {"6", tr.T("report.sev.low")}}, &prio)
	results := sheetNS + `<sheetPr><tabColor rgb="FF00F2FE"/></sheetPr>` +
		fmt.Sprintf(`<dimension ref="A1:H%d"/>`, last) +
		`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft"/></sheetView></sheetViews>` +
		`<sheetFormatPr defaultRowHeight="15"/>` + colsXML(30, 56, 14, 8, 12, 12, 60, 60) +
		`<sheetData>` + rs.b.String() + `</sheetData>` +
		fmt.Sprintf(`<autoFilter ref="A1:H%d"/>`, last) + cf +
		`<pageMargins left="0.5" right="0.5" top="0.6" bottom="0.6" header="0.3" footer="0.3"/><pageSetup orientation="landscape" fitToHeight="0"/></worksheet>`

	// ---- Summary sheet
	var pass, fail, skip int
	sev := map[string]int{}
	var failed []engine.RuleResult
	for _, r := range rep.Results {
		switch r.Status {
		case engine.StatusPass:
			pass++
		case engine.StatusSkip:
			skip++
		default:
			fail++
			sev[sevClass(r.Rule.Severity)]++
			failed = append(failed, r)
		}
	}
	rank := func(s string) int {
		if v, ok := sevRank[sevClass(s)]; ok {
			return v
		}
		return 4
	}
	sort.SliceStable(failed, func(i, j int) bool { return rank(failed[i].Rule.Severity) < rank(failed[j].Rule.Severity) })
	if len(failed) > 15 {
		failed = failed[:15]
	}
	when := rep.Timestamp
	if when.IsZero() {
		when = time.Now()
	}
	var ss sheetBuilder
	ss.open(28)
	ss.text(0, tr.T("report.xlsx.title"), sTitle)
	ss.close()
	ss.open(0)
	ss.text(0, strings.TrimSpace(fmt.Sprintf("%s  %s/%s  %s", rep.Hostname, rep.OS, rep.Arch, when.Local().Format("2006-01-02 15:04 MST"))), sMuted)
	ss.close()
	ss.blank()
	ss.open(36)
	ss.text(0, tr.T("report.score.label"), sLabel)
	ss.num(1, float64(int(score(rep)*10+0.5))/10, sScore)
	ss.close()
	ss.blank()
	ss.open(0)
	ss.text(0, tr.T("report.xlsx.field"), sHeader)
	ss.text(1, tr.T("report.xlsx.value"), sHeader)
	ss.close()
	for _, kv := range []struct {
		k string
		v int
	}{{"report.stat.passed", pass}, {"report.stat.failed", fail}, {"report.stat.skipped", skip}, {"report.stat.total", len(rep.Results)}} {
		ss.open(0)
		ss.text(0, tr.T(kv.k), sLabel)
		ss.num(1, float64(kv.v), sInt)
		ss.close()
	}
	ss.blank()
	ss.open(0)
	ss.text(0, tr.T("report.xlsx.severity"), sHeader)
	ss.text(1, tr.T("report.xlsx.count"), sHeader)
	ss.close()
	sevStart := ss.row + 1
	for _, k := range []string{"critical", "high", "medium", "low", "other"} {
		ss.open(0)
		ss.text(0, tr.T("report.sev."+k), sLabel)
		ss.num(1, float64(sev[k]), sInt)
		ss.close()
	}
	sevEnd := ss.row
	ss.blank()
	ss.open(0)
	ss.text(0, tr.T("report.fixfirst.title"), sLabel)
	ss.close()
	ss.open(0)
	ss.text(0, tr.T("report.xlsx.hdr.id"), sHeader)
	ss.text(1, tr.T("report.xlsx.hdr.title"), sHeader)
	ss.text(2, tr.T("report.xlsx.hdr.severity"), sHeader)
	ss.close()
	topStart := ss.row + 1
	for _, r := range failed {
		ss.open(0)
		ss.text(0, r.Rule.ID, sWrap)
		ss.text(1, r.Rule.Title, sWrap)
		ss.text(2, sevLabel(r.Rule.Severity), sWrap)
		ss.close()
	}
	prio = 0
	cf = cfRules(fmt.Sprintf("A%d:A%d", sevStart, sevEnd), [][2]string{
		{"3", tr.T("report.sev.critical")}, {"4", tr.T("report.sev.high")}, {"5", tr.T("report.sev.medium")}, {"6", tr.T("report.sev.low")}}, &prio)
	if len(failed) > 0 {
		cf += cfRules(fmt.Sprintf("C%d:C%d", topStart, ss.row), [][2]string{
			{"3", tr.T("report.sev.critical")}, {"4", tr.T("report.sev.high")}, {"5", tr.T("report.sev.medium")}, {"6", tr.T("report.sev.low")}}, &prio)
	}
	summary := sheetNS + `<sheetPr><tabColor rgb="FF10B981"/></sheetPr>` +
		`<sheetViews><sheetView showGridLines="0" workbookViewId="0" tabSelected="1"/></sheetViews><sheetFormatPr defaultRowHeight="15"/>` +
		colsXML(34, 60, 14) + `<sheetData>` + ss.b.String() + `</sheetData>` + cf +
		`<pageMargins left="0.5" right="0.5" top="0.6" bottom="0.6" header="0.3" footer="0.3"/></worksheet>`

	sheets := []xlsxSheet{{nameSummary, summary}, {nameResults, results}}

	var wb, rels, ct strings.Builder
	wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView/></bookViews><sheets>`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for i, sh := range sheets {
		fmt.Fprintf(&wb, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlText(sh.name), i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
		fmt.Fprintf(&ct, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	wb.WriteString(`</sheets>`)
	fmt.Fprintf(&wb, `<definedNames><definedName name="_xlnm._FilterDatabase" localSheetId="1" hidden="1">'%s'!$A$1:$H$%d</definedName></definedNames></workbook>`, strings.ReplaceAll(xmlText(nameResults), "'", "''"), last)
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`, len(sheets)+1)
	ct.WriteString(`</Types>`)

	files := []struct{ name, body string }{
		{"[Content_Types].xml", ct.String()},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", wb.String()},
		{"xl/_rels/workbook.xml.rels", rels.String()},
		{"xl/styles.xml", stylesXML},
	}
	for i, sh := range sheets {
		files = append(files, struct{ name, body string }{fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + sh.xml})
	}
	zw := zip.NewWriter(w)
	for _, f := range files {
		fw, err := zw.Create(f.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(fw, f.body); err != nil {
			return err
		}
	}
	return zw.Close()
}
