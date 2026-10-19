// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// tabReport has one rule for every tab, all carrying markup that must stay inert.
func tabReport() *engine.ScanReport {
	evil := `<img src=x onerror=alert(1)>`
	mk := func(id, status string, audit, rem string) engine.RuleResult {
		return engine.RuleResult{
			Rule: rules.Rule{ID: id, Title: "T " + evil, Description: "D " + evil, Severity: "high", Target: "t" + evil, Level: 1,
				Reference: "CIS " + evil + " Benchmark - 1.1", Audit: rules.AuditSpec{Type: audit, Command: "run " + evil},
				Remediation: rules.RemediateSpec{Type: rem, Command: "fix " + evil}},
			Status: engine.Status(status), Details: "out " + evil,
		}
	}
	return &engine.ScanReport{Hostname: "h" + evil, OS: "linux", Arch: "amd64", Results: []engine.RuleResult{
		mk("TMA-<B>X-1.1", "FAIL", "command", "command"),     // quick win
		mk("TMA-DOCKER-1.2", "FAIL", "command", "iac_patch"), // needs planning
		mk("TMA-DOCKER-1.3", "FAIL", "command", ""),          // manual review
		mk("TMA-DOCKER-1.4", "SKIP", "manual", "command"),    // skipped: manual
		mk("TMA-DOCKER-1.5", "SKIP", "command", "command"),   // skipped: not applicable
		mk("TMA-DOCKER-1.6", "PASS", "command", "command"),
	}}
}

func renderHTML(t *testing.T, rep *engine.ScanReport, lang string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteLang(&buf, "html", rep, lang); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestHTMLTabsHaveARIARoles(t *testing.T) {
	out := renderHTML(t, tabReport(), "en-US")
	if !strings.Contains(out, `role="tablist"`) {
		t.Fatal("no tablist")
	}
	for _, id := range []string{"overview", "plan", "results", "benchmarks", "skipped", "about"} {
		for _, want := range []string{
			fmt.Sprintf(`role="tab" id="t-%s"`, id), fmt.Sprintf(`aria-controls="p-%s"`, id),
			fmt.Sprintf(`id="p-%s" class="panel" role="tabpanel" aria-labelledby="t-%s"`, id, id),
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q", want)
			}
		}
	}
	if n := strings.Count(out, `aria-selected="true"`); n != 1 {
		t.Fatalf("exactly one tab must start selected, got %d", n)
	}
	if n := strings.Count(out, `tabindex="-1" data-tab=`); n != 5 {
		t.Fatalf("roving tabindex: %d unselected tabs", n)
	}
}

func TestHTMLEscapesRuleTextInEveryTab(t *testing.T) {
	out := renderHTML(t, tabReport(), "en-US")
	if strings.Contains(out, "<img src=x") || strings.Contains(out, "<B>X") {
		t.Fatal("rule text leaked unescaped")
	}
	for _, p := range []string{"p-plan", "p-results", "p-benchmarks", "p-skipped", "p-overview"} {
		i := strings.Index(out, `id="`+p+`"`)
		if i < 0 {
			t.Fatalf("panel %s missing", p)
		}
		end := len(out)
		if j := strings.Index(out[i+60:], `role="tabpanel"`); j >= 0 {
			end = i + 60 + j
		}
		if !strings.Contains(out[i:end], "&lt;img src=x") && !strings.Contains(out[i:end], "&lt;B&gt;X") {
			t.Errorf("panel %s shows no escaped rule text", p)
		}
	}
	for _, g := range []string{"g-quick", "g-planned", "g-manual"} {
		if !strings.Contains(out, `pgroup `+g) {
			t.Errorf("group %s missing", g)
		}
	}
	// instructions of manual-review rules and reasons of skipped rules are in the Skipped tab
	i := strings.Index(out, `id="p-skipped"`)
	sk := out[i : i+strings.Index(out[i:], `id="p-about"`)]
	if !strings.Contains(sk, "run &lt;img") || !strings.Contains(sk, `class="nareason"`) {
		t.Fatal("skipped tab lacks manual instructions or not-applicable reason")
	}
}

func TestHTMLNoExternalURLsAndCSPHash(t *testing.T) {
	out := renderHTML(t, tabReport(), "vi")
	for _, bad := range []string{`src="http`, `href="http`, "url(http", "@import", "<link ", "fetch(", "XMLHttpRequest"} {
		if strings.Contains(out, bad) {
			t.Fatalf("found %q", bad)
		}
	}
	i := strings.Index(out, "<script>")
	j := i + strings.Index(out[i:], "</script>")
	sum := sha256.Sum256([]byte(out[i+len("<script>") : j]))
	if !strings.Contains(out, "script-src 'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'") {
		t.Fatal("CSP hash mismatch")
	}
	if !strings.Contains(out, "default-src 'none'") || strings.Contains(out, "unsafe-eval") {
		t.Fatal("CSP weakened")
	}
}

func TestHTMLDictionaryHasAll16Locales(t *testing.T) {
	out := renderHTML(t, tabReport(), "en-US")
	m := regexp.MustCompile(`(?s)<script type="application/json" id="i18n-data">(.*?)</script>`).FindStringSubmatch(out)
	if m == nil {
		t.Fatal("dictionary missing")
	}
	var d struct {
		Default string                       `json:"default"`
		Tags    []string                     `json:"tags"`
		M       map[string]map[string]string `json:"m"`
	}
	if err := json.Unmarshal([]byte(m[1]), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.M) != 16 || len(d.Tags) != 16 {
		t.Fatalf("want 16 locales, got %d / %d", len(d.M), len(d.Tags))
	}
	for _, tag := range i18n.Tags {
		if _, ok := d.M[tag]; !ok {
			t.Errorf("dictionary lacks %s", tag)
		}
	}
	for k := range d.M[i18n.Default] {
		if !strings.HasPrefix(k, "report.") || strings.HasPrefix(k, "report.xlsx.") || strings.HasPrefix(k, "report.pdf.") {
			t.Errorf("unexpected key in the browser dictionary: %s", k)
		}
	}
	if len(m[1]) > 700_000 { // ~35 KB per fully translated locale, 15 non-English locales
		t.Errorf("dictionary is %d bytes; keep it compact", len(m[1]))
	}
	if strings.Contains(m[1], "</") {
		t.Fatal("dictionary must not contain a raw closing tag")
	}
}

// every key the template and script use must exist in the en-US catalog
func TestEveryUsedKeyExists(t *testing.T) {
	en := i18n.Messages(i18n.Default)
	has := func(k string) bool {
		if _, ok := en[k]; ok {
			return true
		}
		_, ok := en[k+".other"]
		return ok
	}
	re := regexp.MustCompile(`"(report\.[a-z0-9.]+)"`)
	for _, name := range []string{"report.html.tmpl", "report.js"} {
		for _, m := range re.FindAllStringSubmatch(asset(name), -1) {
			if !strings.HasSuffix(m[1], ".") && !has(m[1]) {
				t.Errorf("%s uses missing key %s", name, m[1])
			}
		}
	}
	for _, s := range []string{"critical", "high", "medium", "low", "other"} {
		for _, k := range []string{"report.sev." + s} {
			if !has(k) {
				t.Errorf("missing %s", k)
			}
		}
	}
	for _, k := range []string{"report.risk.low", "report.risk.service", "report.risk.guidance", "report.risk.database", "report.risk.unknown",
		"report.status.pass", "report.status.fail", "report.status.skip", "report.status.error",
		"report.plan.quick.title", "report.plan.planned.title", "report.plan.manual.title", "report.plan.quick.desc", "report.plan.planned.desc", "report.plan.manual.desc"} {
		if !has(k) {
			t.Errorf("missing %s", k)
		}
	}
	// the xlsx/pdf writers build keys dynamically; make sure those exist too
	for _, h := range []string{"id", "title", "target", "level", "severity", "status", "details", "remediation"} {
		if !has("report.xlsx.hdr." + h) {
			t.Errorf("missing report.xlsx.hdr.%s", h)
		}
	}
}

func TestWriteLangIsGoroutineSafe(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		lang := []string{"en-US", "vi", "ja", "sr-Cyrl"}[i%4]
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, f := range []string{"html", "xlsx", "pdf"} {
				var buf bytes.Buffer
				if err := WriteLang(&buf, f, tabReport(), lang); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if Lang != "" {
		t.Fatalf("WriteLang must not change the package default, Lang=%q", Lang)
	}
}

func TestHTMLLanguageAndDefault(t *testing.T) {
	if out := renderHTML(t, tabReport(), "vi"); !strings.Contains(out, `<html lang="vi"`) {
		t.Fatal("lang attribute not set")
	}
	t.Setenv("TMAEGIS_LANG", "de")
	if out := renderHTML(t, tabReport(), ""); !strings.Contains(out, `<html lang="de"`) {
		t.Fatal("empty lang must use the detected language")
	}
}

func TestActionPlanClassification(t *testing.T) {
	cases := []struct {
		rem, cmd, plan, risk string
	}{
		{"command", "chmod 600 /etc/x", "quick", "low"},
		{"command", "systemctl restart sshd", "planned", "service"},
		{"file_line", "", "quick", "low"},
		{"iac_patch", "edit it", "planned", "guidance"},
		{"sql_exec", "ALTER ...", "planned", "database"},
		{"", "", "manual", "unknown"},
	}
	for _, c := range cases {
		r := engine.RuleResult{Status: engine.StatusFail, Rule: rules.Rule{Remediation: rules.RemediateSpec{Type: c.rem, Command: c.cmd, FilePath: "/etc/x", ReplaceLine: "a=b"}}}
		txt, _ := remediationText(r)
		if plan, risk := classify(r, txt); plan != c.plan || risk != c.risk {
			t.Errorf("%+v -> %s/%s, want %s/%s", c, plan, risk, c.plan, c.risk)
		}
	}
	if plan, _ := classify(engine.RuleResult{Status: engine.StatusErr}, "x"); plan != "manual" {
		t.Error("errors need manual review")
	}
}

func TestBenchKey(t *testing.T) {
	for id, want := range map[string]string{
		"TMA-DOCKER-1.2.3": "DOCKER", "TMA-CISCO-IOS-2.1": "CISCO-IOS", "TMA-LINUX-5": "LINUX", "X": "X", "1.2": "",
	} {
		if got := benchKey(id, ""); got != want {
			t.Errorf("benchKey(%q)=%q want %q", id, got, want)
		}
	}
	if benchKey("TMA-DOCKER-NO-PRIVILEGED", "docker") != "DOCKER" || benchKey("TMA-CISCO-IOS-1.1", "cisco-ios") != "CISCO-IOS" {
		t.Error("rule ids with a word suffix belong to their target's benchmark")
	}
	if benchKey("1.2", "aws") != "AWS" {
		t.Error("target fallback")
	}
}

// the browser's pluralCategory must agree with the Go rules (internal/i18n) for every locale
func goPlural(tag string, n int) string {
	if n < 0 {
		n = -n
	}
	switch tag {
	case "ja", "ko", "zh-Hans", "zh-Hant", "vi", "id":
		return "other"
	case "fr", "pt-BR":
		if n == 0 || n == 1 {
			return "one"
		}
		return "other"
	case "ru", "sr-Latn", "sr-Cyrl":
		m10, m100 := n%10, n%100
		switch {
		case m10 == 1 && m100 != 11:
			return "one"
		case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
			return "few"
		case tag == "ru":
			return "many"
		}
		return "other"
	}
	if n == 1 {
		return "one"
	}
	return "other"
}

func TestPluralMirrorStaysWithinLocaleCategories(t *testing.T) {
	for _, tag := range i18n.Tags {
		allowed := map[string]bool{}
		for _, c := range i18n.PluralCategories(tag) {
			allowed[c] = true
		}
		for n := 0; n < 300; n++ {
			if c := goPlural(tag, n); !allowed[c] {
				t.Fatalf("%s n=%d: category %s not in %v", tag, n, c, i18n.PluralCategories(tag))
			}
		}
		// and it agrees with the real translator
		tr := i18n.New(tag)
		for _, n := range []int{0, 1, 2, 5} {
			if tr.N("report.plan.count", n) == "" {
				t.Fatal("N returned nothing")
			}
		}
	}
}

func TestJSPluralParity(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	js := asset("report.js")
	a, b := strings.Index(js, "/*PLURAL-BEGIN*/"), strings.Index(js, "/*PLURAL-END*/")
	if a < 0 || b < a {
		t.Fatal("plural markers missing from report.js")
	}
	script := js[a:b] + `
var tags = ` + mustJSON(i18n.Tags) + `, out = {};
tags.forEach(function (t) { out[t] = []; for (var n = 0; n < 1200; n++) out[t].push(pluralCategory(t, n)); });
process.stdout.write(JSON.stringify(out));`
	f := filepath.Join(t.TempDir(), "p.js")
	if err := os.WriteFile(f, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command(node, f).Output()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string][]string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, tag := range i18n.Tags {
		for n, c := range got[tag] {
			if want := goPlural(tag, n); c != want {
				t.Fatalf("%s n=%d: js=%s go=%s", tag, n, c, want)
			}
		}
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
