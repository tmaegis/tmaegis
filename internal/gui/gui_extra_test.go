// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tmaegis/tmaegis/internal/rules"
)

const deviceRule = `id: D.1
title: No telnet
description: device rule
target: cisco-ios
level: 1
severity: medium
audit:
  type: config_regex
  pattern: "(?m)^transport input ssh$"
`

func scanDone(t *testing.T, e *env, p ScanParams) JobView {
	t.Helper()
	c, b := e.do(t, "POST", "/api/scan", p, nil)
	if c != 202 {
		t.Fatalf("scan: %d %s", c, b)
	}
	var j JobView
	_ = json.Unmarshal(b, &j)
	return waitJob(t, e, j.ID, stateDone, stateFailed, stateCancelled)
}

func TestSessionListsFormatsAndBeta(t *testing.T) {
	e := newEnv(t, false)
	_, b := e.do(t, "GET", "/api/session", nil, nil)
	var s struct {
		Formats []string
		Beta    []string
	}
	_ = json.Unmarshal(b, &s)
	if !slices.Contains(s.Formats, "pdf") || !slices.Contains(s.Formats, "sarif") {
		t.Errorf("formats: %v", s.Formats)
	}
	if slices.Contains(s.Beta, "en-US") || !slices.Contains(s.Beta, "vi") {
		t.Errorf("beta: %v", s.Beta)
	}
}

func TestPlatformsEndpoint(t *testing.T) {
	e := newEnv(t, false)
	c, b := e.do(t, "GET", "/api/platforms", nil, nil)
	var d struct {
		Categories []platformCategory
		Rules      int
		Targets    int
	}
	if c != 200 || json.Unmarshal(b, &d) != nil {
		t.Fatalf("platforms: %d %s", c, b)
	}
	if d.Rules != 1 || d.Targets != 1 || len(d.Categories) != 1 || d.Categories[0].ID != "os" {
		t.Fatalf("platforms: %s", b)
	}
	tg := d.Categories[0].Groups[0].Targets[0]
	if tg.ID != "linux" || tg.Name != "Linux distributions" || tg.Total != 1 || tg.Auto != 1 || tg.Fixable != 1 || !tg.Detected || tg.Severity["high"] != 1 {
		t.Errorf("linux: %+v", tg)
	}
	if c, _ := e.do(t, "GET", "/api/platforms?rules_dir=/no/such/dir", nil, nil); c != 400 {
		t.Errorf("bad dir: %d", c)
	}
	// /api/targets keeps its old shape
	_, b = e.do(t, "GET", "/api/targets", nil, nil)
	if !strings.Contains(string(b), `{"name":"linux","count":1}`) {
		t.Errorf("targets: %s", b)
	}
}

// The catalogue mirrors the website: every target maps to a known category and group, and an
// unknown target falls back to "other" instead of disappearing.
func TestPlatformCatalog(t *testing.T) {
	if len(platformCatalog) != 96 {
		t.Errorf("catalogue has %d targets, the website lists 96", len(platformCatalog))
	}
	for id, m := range platformCatalog {
		if !slices.Contains(categoryGroups[m.category], m.group) {
			t.Errorf("%s: group %s not in category %s", id, m.group, m.category)
		}
	}
	st := statsOf([]rules.Rule{{ID: "x", Target: "brand-new", Severity: "low", Level: 1, Audit: rules.AuditSpec{Type: "manual"}}})
	g := groupPlatforms(st, nil)
	if len(g) != 1 || g[0].ID != "other" || g[0].Groups[0].Targets[0].Manual != 1 {
		t.Errorf("unknown target: %+v", g)
	}
}

func TestDashboardAndScanStream(t *testing.T) {
	e := newEnv(t, false)
	_, b := e.do(t, "GET", "/api/dashboard", nil, nil)
	var d0 map[string]any
	_ = json.Unmarshal(b, &d0)
	if d0["last"] != nil || d0["benchmarks"] == nil || d0["catalog"] == nil {
		t.Fatalf("empty dashboard: %s", b)
	}
	v := scanDone(t, e, ScanParams{Scope: "dir", Level: 1})
	if len(v.Log) != 1 || v.Log[0].Seq != 1 || v.Log[0].ID != "T.1" || v.Log[0].Status != "FAIL" {
		t.Errorf("stream: %+v", v.Log)
	}
	_, b = e.do(t, "GET", "/api/jobs", nil, nil)
	if strings.Contains(string(b), `"log"`) {
		t.Errorf("job list must not carry the stream: %s", b)
	}
	_, b = e.do(t, "GET", "/api/dashboard", nil, nil)
	var d struct {
		Last       ReportSummary
		Severity   map[string]sevRow
		TopFailing []catFail `json:"top_failing"`
		Fixable    int
		Trend      []map[string]any
		Benchmarks []map[string]any
	}
	if json.Unmarshal(b, &d) != nil || d.Last.ID != v.ReportID || d.Severity["high"].Fail != 1 || d.Fixable != 1 || len(d.Trend) != 1 {
		t.Fatalf("dashboard: %s", b)
	}
	if len(d.TopFailing) != 1 || d.TopFailing[0].ID != "linux" || d.TopFailing[0].Category != "os" {
		t.Errorf("top failing: %+v", d.TopFailing)
	}
	if len(d.Benchmarks) == 0 || d.Benchmarks[0]["id"] != "linux" || d.Benchmarks[0]["rules"].(float64) != 1 {
		t.Errorf("benchmarks: %+v", d.Benchmarks)
	}
}

func TestScanPlan(t *testing.T) {
	e := newEnv(t, false)
	_ = os.WriteFile(filepath.Join(e.s.opts.RulesDir, "d1.yaml"), []byte(deviceRule), 0o644)
	c, b := e.do(t, "POST", "/api/scan/plan", ScanParams{Scope: "dir", Level: 1}, nil)
	var p struct {
		Total, Auto, Device, Fixable int
		DeviceAvailable              int `json:"device_available"`
		Severity                     map[string]int
		Targets                      []planTarget
	}
	if c != 200 || json.Unmarshal(b, &p) != nil || p.Total != 1 || p.Auto != 1 || p.Fixable != 1 || p.Severity["high"] != 1 || p.Targets[0].ID != "linux" {
		t.Fatalf("plan dir: %d %s", c, b)
	}
	// a network device target without a configuration runs nothing, but says what a file would enable
	_, b = e.do(t, "POST", "/api/scan/plan", ScanParams{Scope: "target", Target: "cisco-ios"}, nil)
	p.DeviceAvailable, p.Total = 0, -1
	_ = json.Unmarshal(b, &p)
	if p.Total != 0 || p.DeviceAvailable != 1 {
		t.Errorf("plan device: %s", b)
	}
	// several targets at once
	_, b = e.do(t, "POST", "/api/scan/plan", ScanParams{Scope: "target", Target: "linux, cisco-ios"}, nil)
	_ = json.Unmarshal(b, &p)
	if p.Total != 1 {
		t.Errorf("plan multi: %s", b)
	}
	if c, _ := e.do(t, "POST", "/api/scan/plan", ScanParams{Scope: "nope"}, nil); c != 400 {
		t.Errorf("bad scope: %d", c)
	}
}

func TestDeviceConfigScan(t *testing.T) {
	e := newEnv(t, false)
	_ = os.WriteFile(filepath.Join(e.s.opts.RulesDir, "d1.yaml"), []byte(deviceRule), 0o644)
	if c, _ := e.do(t, "POST", "/api/scan", ScanParams{Scope: "target", Target: "cisco-ios", DeviceConfig: "dev-1"}, nil); c != 400 {
		t.Errorf("unknown device config: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/device-config", deviceReq{Name: "x.txt"}, nil); c != 400 {
		t.Errorf("empty config: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/device-config", deviceReq{Name: "big.txt", Content: strings.Repeat("a", maxDeviceConfig+1)}, nil); c != 413 {
		t.Errorf("too large: %d", c)
	}
	c, b := e.do(t, "POST", "/api/device-config", deviceReq{Name: "../../router.txt", Content: "hostname r1\nline vty 0 4\ntransport input ssh\n"}, nil)
	var dc struct {
		ID, Name    string
		Size, Lines int
	}
	if c != 200 || json.Unmarshal(b, &dc) != nil || dc.Name != "router.txt" || dc.Lines != 3 || !devRe.MatchString(dc.ID) {
		t.Fatalf("upload: %d %s", c, b)
	}
	if p := e.s.store.deviceConfigPath(dc.ID); p == "" || !strings.HasPrefix(p, e.s.store.dir) {
		t.Fatalf("stored at %q", p)
	}
	v := scanDone(t, e, ScanParams{Scope: "target", Target: "cisco-ios", DeviceConfig: dc.ID, DeviceName: dc.Name})
	if v.State != stateDone || v.Total != 1 || v.Passed != 1 {
		t.Fatalf("device scan: %+v", v)
	}
	sr, _ := e.s.store.report(v.ReportID)
	if sr.Params.DeviceName != "router.txt" || sr.Report.Results[0].Rule.ID != "D.1" {
		t.Errorf("stored params: %+v", sr.Params)
	}
	if e.s.store.deviceConfigPath("../etc/passwd") != "" {
		t.Error("path traversal accepted")
	}
}

func TestSamples(t *testing.T) {
	e := newEnv(t, false)
	base := filepath.Dir(e.s.opts.RulesDir) // samples/ sits next to rules/
	_ = os.MkdirAll(filepath.Join(base, "samples", "network", "cisco-ios"), 0o755)
	_ = os.WriteFile(filepath.Join(base, "samples", "network", "cisco-ios", "nc.txt"), []byte("transport input telnet\n"), 0o644)
	_ = os.WriteFile(filepath.Join(base, "samples", "manifest.json"), []byte(`[{"target":"cisco-ios","compliant":"network/cisco-ios/c.txt","noncompliant":"network/cisco-ios/nc.txt","rules":1}]`), 0o644)
	_, b := e.do(t, "GET", "/api/samples", nil, nil)
	if !strings.Contains(string(b), `"found":true`) || !strings.Contains(string(b), "cisco-ios") {
		t.Fatalf("samples: %s", b)
	}
	c, b := e.do(t, "POST", "/api/device-config", deviceReq{Sample: "cisco-ios", Variant: "noncompliant"}, nil)
	if c != 200 || !strings.Contains(string(b), `"name":"nc.txt"`) {
		t.Errorf("sample: %d %s", c, b)
	}
	if c, _ := e.do(t, "POST", "/api/device-config", deviceReq{Sample: "cisco-ios", Variant: "compliant"}, nil); c != 404 {
		t.Errorf("missing sample file: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/device-config", deviceReq{Sample: "../../etc"}, nil); c != 404 {
		t.Errorf("unknown sample: %d", c)
	}
}

func TestCompareScans(t *testing.T) {
	e := newEnv(t, false)
	a := scanDone(t, e, ScanParams{Scope: "dir", Level: 1})
	_ = os.WriteFile(e.marker, []byte("ok\n"), 0o600)
	b := scanDone(t, e, ScanParams{Scope: "dir", Level: 1})
	for _, q := range []string{"?a=" + a.ReportID + "&b=" + b.ReportID, "?a=" + b.ReportID + "&b=" + a.ReportID} {
		c, body := e.do(t, "GET", "/api/compare"+q, nil, nil)
		var d struct {
			Before, After ReportSummary
			Delta         float64
			Counts        map[string]int
			Lists         map[string][]cmpRow
		}
		if c != 200 || json.Unmarshal(body, &d) != nil {
			t.Fatalf("compare: %d %s", c, body)
		}
		if d.Before.ID != a.ReportID || d.Counts["fixed"] != 1 || d.Delta != 100 || d.Lists["fixed"][0].Before != "FAIL" || d.Lists["fixed"][0].After != "PASS" {
			t.Errorf("compare %s: %s", q, body)
		}
	}
	if c, _ := e.do(t, "GET", "/api/compare?a=scan-1&b="+b.ReportID, nil, nil); c != 404 {
		t.Errorf("unknown: %d", c)
	}
}

func TestFixPreviewDiffAndRisk(t *testing.T) {
	e := newEnv(t, false)
	v := scanDone(t, e, ScanParams{Scope: "dir", Level: 1})
	c, b := e.do(t, "POST", "/api/fix/preview", fixReq{ReportID: v.ReportID, RuleIDs: []string{"T.1"}}, nil)
	var p struct{ Items []FixItem }
	if c != 200 || json.Unmarshal(b, &p) != nil || len(p.Items) != 1 {
		t.Fatalf("preview: %d %s", c, b)
	}
	it := p.Items[0]
	if it.Diff == nil || filepath.Clean(it.Diff.File) != filepath.Clean(e.marker) || it.Risk == "" {
		t.Fatalf("item: %+v", it)
	}
	var del, add bool
	for _, l := range it.Diff.Lines {
		del = del || (l.Op == "-" && l.Text == "bad" && l.Old == 1)
		add = add || (l.Op == "+" && l.Text == "ok" && l.New == 1)
	}
	if !del || !add {
		t.Errorf("diff: %+v", it.Diff.Lines)
	}
}

func TestLineDiff(t *testing.T) {
	a := strings.Split("a b c d e f g h i j k l m n", " ")
	b := strings.Split("a b c X e f g h i j k l m n o", " ")
	out, trunc := lineDiff(a, b)
	if trunc {
		t.Fatal("truncated")
	}
	var ops strings.Builder
	for _, l := range out {
		ops.WriteString(l.Op)
	}
	// context around the replacement of d by X, a gap, then the appended k
	if got := ops.String(); got != "   -+   @   +" || out[3].Old != 4 || out[4].New != 4 || out[len(out)-1].New != 15 {
		t.Errorf("ops %q: %+v", got, out)
	}
	if _, trunc := lineDiff(nil, make([]string, diffMaxLines+10)); !trunc {
		t.Error("long diff not truncated")
	}
}

func TestRiskOf(t *testing.T) {
	cases := []struct {
		rem  rules.RemediateSpec
		want string
	}{
		{rules.RemediateSpec{Type: "file_line", FilePath: "/etc/login.defs", VerifyCmd: "true"}, "low"},
		{rules.RemediateSpec{Type: "file_line", FilePath: "/etc/login.defs"}, "medium"},
		{rules.RemediateSpec{Type: "file_line", FilePath: "/etc/ssh/sshd_config", VerifyCmd: "sshd -t -f {file}"}, "high"},
		{rules.RemediateSpec{Type: "command", Command: "chmod 600 /etc/x"}, "medium"},
		{rules.RemediateSpec{Type: "command", Command: "systemctl disable --now avahi-daemon"}, "high"},
		{rules.RemediateSpec{Type: "iac_patch", Command: "systemctl"}, "low"},
	}
	for _, c := range cases {
		if got, _ := riskOf(rules.Rule{Severity: "low", Remediation: c.rem}); got != c.want {
			t.Errorf("%+v: %s, want %s", c.rem, got, c.want)
		}
	}
}
