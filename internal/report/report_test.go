// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

func sample() *engine.ScanReport {
	return &engine.ScanReport{Results: []engine.RuleResult{
		{Rule: rules.Rule{ID: "P", Title: "pass", Severity: "low"}, Status: engine.StatusPass},
		{Rule: rules.Rule{ID: "F", Title: "fail", Severity: "critical"}, Status: engine.StatusFail, Details: "bad"},
	}}
}

func TestSARIF(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "sarif", sample()); err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	run := log.Runs[0]
	if log.Version != "2.1.0" || len(run.Tool.Driver.Rules) != 2 || len(run.Results) != 1 {
		t.Fatalf("unexpected SARIF: %s", buf.String())
	}
	if r := run.Results[0]; r.RuleID != "F" || r.Level != "error" {
		t.Fatalf("bad result: %+v", r)
	}
}

func TestJSONAndUnsupported(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "json", sample()); err != nil || !json.Valid(buf.Bytes()) {
		t.Fatalf("json failed: %v", err)
	}
	if err := Write(&buf, "xml", sample()); err == nil {
		t.Fatal("unsupported format must error")
	}
}

func evilReport() *engine.ScanReport {
	return &engine.ScanReport{OS: "linux", Arch: "amd64", TotalRules: 2, PassedCount: 1, FailedCount: 1, Results: []engine.RuleResult{
		{Rule: rules.Rule{ID: "P", Title: "ok (paren) \\ slash", Severity: "low", Target: "linux", Level: 1}, Status: engine.StatusPass},
		{Rule: rules.Rule{ID: "F", Title: "<script>alert(1)</script>", Severity: "high", Target: "linux", Level: 1}, Status: engine.StatusFail, Details: "=HYPERLINK(\"http://evil\")"},
	}}
}

func TestCSVNeutralisesFormulas(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "csv", evilReport()); err != nil {
		t.Fatal(err)
	}
	rec, err := csv.NewReader(&buf).ReadAll()
	if err != nil || len(rec) != 3 {
		t.Fatalf("csv: %v %v", rec, err)
	}
	if got := rec[2][6]; got != "'=HYPERLINK(\"http://evil\")" {
		t.Fatalf("formula not neutralised: %q", got)
	}
}

func TestHTMLEscapes(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "html", evilReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)") || !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("html not escaped: %s", out)
	}
	if !strings.Contains(out, "50.0%") {
		t.Fatal("score missing")
	}
}

func TestJSONLAndASFF(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "jsonl", evilReport()); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(&buf)
	n := 0
	for sc.Scan() {
		var m map[string]interface{}
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("jsonl lines = %d", n)
	}

	buf.Reset()
	if err := Write(&buf, "asff", evilReport()); err != nil {
		t.Fatal(err)
	}
	var findings []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &findings); err != nil || len(findings) != 2 {
		t.Fatalf("asff: %v", err)
	}
	f := findings[1]
	if f["SchemaVersion"] != "2018-10-08" || f["Compliance"].(map[string]interface{})["Status"] != "FAILED" ||
		f["Severity"].(map[string]interface{})["Label"] != "HIGH" {
		t.Fatalf("bad ASFF finding: %v", f)
	}
}

func TestXLSXIsValidWorkbook(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLang(&buf, "xlsx", evilReport(), "en-US"); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(data)
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			if _, err := dec.Token(); err != nil {
				if err != io.EOF {
					t.Fatalf("%s is not well-formed XML: %v", f.Name, err)
				}
				break
			}
		}
	}
	for _, want := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if parts[want] == "" {
			t.Fatalf("missing part %s", want)
		}
	}
	res := parts["xl/worksheets/sheet2.xml"]
	for _, want := range []string{"<autoFilter ref=\"A1:H3\"", `state="frozen"`, "<cols>", "conditionalFormatting", "tabColor", "wrapText"} {
		if !strings.Contains(res+parts["xl/styles.xml"], want) {
			t.Fatalf("results sheet lacks %s", want)
		}
	}
	if !strings.Contains(res, "&lt;script&gt;") || strings.Contains(res, "<script>") {
		t.Fatal("sheet content not escaped")
	}
	if !strings.Contains(parts["xl/worksheets/sheet1.xml"], "50") || !strings.Contains(parts["xl/workbook.xml"], `name="Summary"`) {
		t.Fatal("summary sheet missing score or name")
	}
}

func TestXLSXLocalisedSheetNamesAreLegal(t *testing.T) {
	used := map[string]bool{}
	n := sheetName("a/b:c*d?e[f]g\\h 0123456789012345678901234567890123", used)
	if len([]rune(n)) > 31 || strings.ContainsAny(n, "[]:*?/\\") {
		t.Fatalf("illegal sheet name %q", n)
	}
	if n2 := sheetName(n, used); n2 == n {
		t.Fatal("duplicate sheet names must be made unique")
	}
}

func TestPDFStructure(t *testing.T) {
	rep := evilReport()
	for i := 0; i < 120; i++ { // force multiple pages
		rep.Results = append(rep.Results, engine.RuleResult{Rule: rules.Rule{ID: "R", Title: "rule", Severity: "low"}, Status: engine.StatusPass})
	}
	var buf bytes.Buffer
	if err := Write(&buf, "pdf", rep); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatal("bad PDF framing")
	}
	// Every xref entry must point exactly at its "N 0 obj" header.
	idx := bytes.LastIndex(b, []byte("startxref\n"))
	var xrefOff int
	fmt.Sscanf(string(b[idx+len("startxref\n"):]), "%d", &xrefOff)
	table := string(b[xrefOff:])
	var count int
	fmt.Sscanf(table, "xref\n0 %d\n", &count)
	lines := strings.Split(table, "\n")[2 : 2+count]
	for i, l := range lines[1:] {
		var off int
		fmt.Sscanf(l, "%d", &off)
		if !bytes.HasPrefix(b[off:], []byte(fmt.Sprintf("%d 0 obj", i+1))) {
			t.Fatalf("xref entry %d points at %q", i+1, b[off:off+12])
		}
	}
	if !bytes.Contains(b, []byte("/Count 4")) && !bytes.Contains(b, []byte("/Count 5")) {
		t.Fatalf("expected a cover page plus several table pages")
	}
	if !bytes.Contains(b, []byte("/Subtype /CIDFontType2")) || !bytes.Contains(b, []byte("/ToUnicode")) {
		t.Fatal("PDF must embed a TrueType font with a ToUnicode map")
	}
	if p, err := exec.LookPath("pdftotext"); err == nil {
		f := filepath.Join(t.TempDir(), "r.pdf")
		os.WriteFile(f, b, 0o600)
		out, err := exec.Command(p, f, "-").CombinedOutput()
		if err != nil || !strings.Contains(string(out), "Compliance Report") || !strings.Contains(string(out), "ok (paren) \\ slash") {
			t.Fatalf("pdftotext rejected PDF or text missing: %v %s", err, out)
		}
	}
}

func TestPDFCJKFallsBackToEnglishWithNote(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLang(&buf, "pdf", evilReport(), "ja"); err != nil || buf.Len() < 1000 {
		t.Fatalf("ja pdf: %v (%d bytes)", err, buf.Len())
	}
	if p, err := exec.LookPath("pdftotext"); err == nil {
		f := filepath.Join(t.TempDir(), "r.pdf")
		os.WriteFile(f, buf.Bytes(), 0o600)
		out, _ := exec.Command(p, f, "-").CombinedOutput()
		if !strings.Contains(string(out), "HTML report and use Print / PDF") {
			t.Fatalf("CJK note missing: %s", out)
		}
	}
}

func TestHTMLReportIsSelfContainedAndSafe(t *testing.T) {
	rep := evilReport()
	rep.Results = append(rep.Results,
		engine.RuleResult{Rule: rules.Rule{ID: "C", Title: `quote " and 'apos' <b>`, Severity: "critical", Target: "linux", Level: 2,
			Remediation: rules.RemediateSpec{Type: "iac_patch", Command: "Edit the config"}}, Status: engine.StatusFail, Details: "x"},
		engine.RuleResult{Rule: rules.Rule{ID: "S", Title: "skipped", Severity: "low", Target: "linux", Level: 1}, Status: engine.StatusSkip})
	var buf bytes.Buffer
	if err := Write(&buf, "html", rep); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, bad := range []string{`src="http`, `href="http`, "url(http", "@import", "<link "} {
		if strings.Contains(out, bad) {
			t.Fatalf("report must not load anything external, found %q", bad)
		}
	}
	// the CSP allows exactly the one inline script that ships in the file
	i := strings.Index(out, "<script>")
	if i < 0 {
		t.Fatal("script missing")
	}
	j := i + strings.Index(out[i:], "</script>")
	sum := sha256.Sum256([]byte(out[i+len("<script>") : j]))
	if !strings.Contains(out, "script-src 'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'") {
		t.Fatal("CSP hash does not match the embedded script")
	}
	if !strings.Contains(out, "&lt;b&gt;") || strings.Contains(out, `data-title="quote "`) {
		t.Fatal("rule text must be escaped, including inside attributes")
	}
	// "fix these first" lists the critical failure before the high one
	if a, b := strings.Index(out, `data-jump="3"`), strings.Index(out, `data-jump="2"`); a < 0 || b < 0 || a > b {
		t.Fatalf("top list order wrong: %d %d", a, b)
	}
	for _, want := range []string{"Guidance only", "font/woff2", "Geist Mono", `data-status="SKIP"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
}
