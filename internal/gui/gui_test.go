// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/report"
)

const testRule = `id: T.1
title: Marker file exists
description: test rule
target: linux
level: 1
severity: high
audit:
  type: file_regex
  file_path: %s
  pattern: "(?m)^ok$"
remediation:
  type: file_line
  file_path: %s
  match_regex: "(?m)^bad$"
  replace_line: "ok"
  backup_files: [%s]
`

type env struct {
	s      *Server
	base   string
	client *http.Client
	marker string
}

func newEnv(t *testing.T, elevated bool) *env {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")
	if err := os.WriteFile(marker, []byte("bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rulesDir := filepath.Join(dir, "rules")
	_ = os.MkdirAll(rulesDir, 0o755)
	y := strings.NewReplacer("\\", "/").Replace(testRule)
	m := filepath.ToSlash(marker)
	_ = os.WriteFile(filepath.Join(rulesDir, "t1.yaml"), []byte(strings.ReplaceAll(y, "%s", m)), 0o644)
	s, err := New(Options{DataDir: filepath.Join(dir, "data"), RulesDir: rulesDir, SnapshotDir: filepath.Join(dir, "snaps"),
		Concurrency: 2, Lang: "en-US", IsElevated: func() bool { return elevated },
		DetectHost: func() discovery.Host { return discovery.Host{OS: "linux", Family: "x"} }})
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.Handler()}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return &env{s: s, base: "http://" + l.Addr().String(), client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, marker: marker}
}

func (e *env) do(t *testing.T, method, path string, body any, mod func(*http.Request)) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.base+path, rd)
	req.Header.Set("Authorization", "Bearer "+e.s.Token())
	if method != http.MethodGet {
		req.Header.Set(nonceHeader, e.s.Nonce())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", e.base)
	}
	if mod != nil {
		mod(req)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestAuthHostOriginNonce(t *testing.T) {
	e := newEnv(t, false)
	if c, _ := e.do(t, "GET", "/api/session", nil, func(r *http.Request) { r.Header.Del("Authorization") }); c != 401 {
		t.Errorf("no token: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/session", nil, func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }); c != 401 {
		t.Errorf("bad token: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/session", nil, func(r *http.Request) { r.Host = "evil.example:" + e.base[strings.LastIndex(e.base, ":")+1:] }); c != 403 {
		t.Errorf("rebinding host: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/session", nil, nil); c != 200 {
		t.Errorf("valid: %d", c)
	}
	// token exchange sets a cookie and redirects, and the page then loads with the cookie
	resp, _ := e.client.Get(e.base + "/?t=" + e.s.Token())
	if resp.StatusCode != 303 || len(resp.Cookies()) == 0 || !resp.Cookies()[0].HttpOnly {
		t.Fatalf("exchange: %d %v", resp.StatusCode, resp.Cookies())
	}
	req, _ := http.NewRequest("GET", e.base+"/", nil)
	req.AddCookie(resp.Cookies()[0])
	r2, _ := e.client.Do(req)
	b, _ := io.ReadAll(r2.Body)
	if r2.StatusCode != 200 || !bytes.Contains(b, []byte("app.js")) || !strings.Contains(r2.Header.Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("index: %d csp=%q", r2.StatusCode, r2.Header.Get("Content-Security-Policy"))
	}
	if c, _ := e.client.Get(e.base + "/"); c.StatusCode != 401 {
		t.Errorf("index without cookie: %d", c.StatusCode)
	}
	// CSRF: cross-origin, missing nonce, form content type
	if c, _ := e.do(t, "POST", "/api/settings", map[string]string{}, func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }); c != 403 {
		t.Errorf("cross-origin POST: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/settings", map[string]string{}, func(r *http.Request) { r.Header.Set(nonceHeader, "WRONG") }); c != 403 {
		t.Errorf("bad nonce: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/settings", map[string]string{}, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); c != 415 {
		t.Errorf("content type: %d", c)
	}
	cookieReq := func(r *http.Request) {
		r.Header.Del("Authorization")
		r.AddCookie(&http.Cookie{Name: cookieName, Value: e.s.Token()})
		r.Header.Del("Origin")
	}
	if c, _ := e.do(t, "POST", "/api/settings", map[string]string{}, cookieReq); c != 403 {
		t.Errorf("cookie POST without Origin: %d", c)
	}
}

func TestI18nEndpoint(t *testing.T) {
	e := newEnv(t, false)
	c, b := e.do(t, "GET", "/api/i18n/en-US", nil, nil)
	var d struct{ Messages map[string]string }
	if c != 200 || json.Unmarshal(b, &d) != nil || d.Messages["gui.nav.scan"] != "Scan" {
		t.Fatalf("i18n: %d %s", c, b)
	}
	for k := range d.Messages {
		if !strings.HasPrefix(k, "gui.") {
			t.Errorf("non-gui key leaked: %s", k)
		}
	}
	if c, _ := e.do(t, "GET", "/api/i18n/xx", nil, nil); c != 404 {
		t.Errorf("unknown tag: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/i18n/vi", nil, nil); c != 200 {
		t.Errorf("vi: %d", c)
	}
}

func waitJob(t *testing.T, e *env, id string, states ...string) JobView {
	t.Helper()
	for i := 0; i < 200; i++ {
		_, b := e.do(t, "GET", "/api/jobs/"+id, nil, nil)
		var v JobView
		_ = json.Unmarshal(b, &v)
		for _, s := range states {
			if v.State == s {
				return v
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %v", id, states)
	return JobView{}
}

func TestScanLifecycleReportsAndFix(t *testing.T) {
	e := newEnv(t, true)
	c, b := e.do(t, "POST", "/api/scan", ScanParams{Scope: "dir", Level: 1}, nil)
	if c != 202 {
		t.Fatalf("scan: %d %s", c, b)
	}
	var j JobView
	_ = json.Unmarshal(b, &j)
	v := waitJob(t, e, j.ID, stateDone, stateFailed)
	if v.State != stateDone || v.Total != 1 || v.Done != 1 || v.Failed != 1 || v.ReportID == "" {
		t.Fatalf("job: %+v", v)
	}
	// SSE of a finished job delivers one terminal event and closes
	c, b = e.do(t, "GET", "/api/jobs/"+j.ID+"/events", nil, nil)
	if c != 200 || !bytes.Contains(b, []byte(`"state":"done"`)) {
		t.Errorf("sse: %d %s", c, b)
	}
	// reports
	c, b = e.do(t, "GET", "/api/report/"+v.ReportID+".json", nil, nil)
	if c != 200 || !bytes.Contains(b, []byte("T.1")) {
		t.Errorf("json report: %d %s", c, b)
	}
	c, b = e.do(t, "GET", "/api/report/"+v.ReportID+".csv", nil, nil)
	if c != 200 || !bytes.Contains(b, []byte("T.1")) {
		t.Errorf("csv report: %d", c)
	}
	c, b = e.do(t, "GET", "/api/report/"+v.ReportID+".html", nil, nil)
	if c != 200 || !bytes.Contains(b, []byte("<html")) {
		t.Errorf("html report: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/report/"+v.ReportID+".exe", nil, nil); c != 400 {
		t.Errorf("bad format: %d", c)
	}
	if c, _ := e.do(t, "GET", "/api/report/..%2f..%2fetc.json", nil, nil); c != 404 {
		t.Errorf("traversal: %d", c)
	}
	// history
	_, b = e.do(t, "GET", "/api/history", nil, nil)
	if !bytes.Contains(b, []byte(v.ReportID)) || !bytes.Contains(b, []byte("scan.done")) {
		t.Errorf("history: %s", b)
	}

	// apply requires a preview and the confirmation code
	req := fixReq{ReportID: v.ReportID, RuleIDs: []string{"T.1"}, ConfirmCode: e.s.Nonce(), DeadmanSec: 15}
	if c, _ := e.do(t, "POST", "/api/fix/apply", req, nil); c != 428 {
		t.Errorf("apply without preview: %d", c)
	}
	c, b = e.do(t, "POST", "/api/fix/preview", req, nil)
	if c != 200 || !bytes.Contains(b, []byte("dry-run")) {
		t.Fatalf("preview: %d %s", c, b)
	}
	if got, _ := os.ReadFile(e.marker); string(got) != "bad\n" {
		t.Fatalf("dry run changed the file: %q", got)
	}
	bad := req
	bad.ConfirmCode = "NOPE"
	if c, _ := e.do(t, "POST", "/api/fix/apply", bad, nil); c != 403 {
		t.Errorf("bad code: %d", c)
	}
	c, b = e.do(t, "POST", "/api/fix/apply", req, nil)
	if c != 202 {
		t.Fatalf("apply: %d %s", c, b)
	}
	_ = json.Unmarshal(b, &j)
	v = waitJob(t, e, j.ID, stateAwaiting, stateFailed)
	if v.State != stateAwaiting || v.SnapshotID == "" || v.Deadline == nil {
		t.Fatalf("apply job: %+v", v)
	}
	if got, _ := os.ReadFile(e.marker); string(got) != "ok\n" {
		t.Fatalf("not applied: %q", got)
	}
	// dead-man: roll back instead of confirming
	e.do(t, "POST", "/api/jobs/"+j.ID+"/rollback", map[string]string{}, nil)
	v = waitJob(t, e, j.ID, stateRolled, stateFailed)
	if v.State != stateRolled {
		t.Fatalf("rollback: %+v", v)
	}
	if got, _ := os.ReadFile(e.marker); string(got) != "bad\n" {
		t.Errorf("not restored: %q", got)
	}
	// snapshot restore needs the code
	_, b = e.do(t, "GET", "/api/snapshots", nil, nil)
	var sn struct{ Snapshots []snapInfo }
	_ = json.Unmarshal(b, &sn)
	if len(sn.Snapshots) != 1 {
		t.Fatalf("snapshots: %s", b)
	}
	if c, _ := e.do(t, "POST", "/api/snapshots/"+sn.Snapshots[0].ID+"/restore", fixReq{ConfirmCode: "x"}, nil); c != 403 {
		t.Errorf("restore bad code: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/snapshots/"+sn.Snapshots[0].ID+"/restore", fixReq{ConfirmCode: e.s.Nonce()}, nil); c != 200 {
		t.Errorf("restore: %d", c)
	}
}

func TestDeadmanTimerRollsBack(t *testing.T) {
	e := newEnv(t, true)
	e.s.opts.DeadmanSec = 1
	_, b := e.do(t, "POST", "/api/scan", ScanParams{Scope: "dir", Level: 1}, nil)
	var j JobView
	_ = json.Unmarshal(b, &j)
	v := waitJob(t, e, j.ID, stateDone)
	req := fixReq{ReportID: v.ReportID, RuleIDs: []string{"T.1"}, ConfirmCode: e.s.Nonce()}
	e.do(t, "POST", "/api/fix/preview", req, nil)
	// bypass the 15 s minimum clamp to keep the test fast
	todo, _, _, _ := e.s.pickFixes(req.ReportID, req.RuleIDs)
	job := e.s.jobs.create("apply")
	go e.s.runApply(job, todo, 700*time.Millisecond)
	waitJob(t, e, job.view().ID, stateRolled, stateFailed)
	if got, _ := os.ReadFile(e.marker); string(got) != "bad\n" {
		t.Errorf("timer did not roll back: %q", got)
	}
}

func TestApplyRefusedWhenNotElevated(t *testing.T) {
	e := newEnv(t, false)
	req := fixReq{ReportID: "scan-1", RuleIDs: []string{"T.1"}, ConfirmCode: e.s.Nonce()}
	if c, _ := e.do(t, "POST", "/api/fix/apply", req, nil); c != 403 {
		t.Errorf("apply: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/snapshots/snap-1/restore", req, nil); c != 403 {
		t.Errorf("restore: %d", c)
	}
}

func TestScanCancel(t *testing.T) {
	e := newEnv(t, false)
	rd := filepath.Join(filepath.Dir(e.marker), "rules")
	slow := "id: S.1\ntitle: slow\ndescription: d\ntarget: linux\nlevel: 1\nseverity: low\naudit:\n  type: command\n  command: sleep 5\n"
	_ = os.WriteFile(filepath.Join(rd, "s1.yaml"), []byte(slow), 0o644)
	_, b := e.do(t, "POST", "/api/scan", ScanParams{Scope: "dir", Level: 1}, nil)
	var j JobView
	_ = json.Unmarshal(b, &j)
	if c, _ := e.do(t, "POST", "/api/scan", ScanParams{Scope: "dir"}, nil); c != 409 {
		t.Errorf("second scan while running: %d", c)
	}
	time.Sleep(150 * time.Millisecond)
	e.do(t, "POST", "/api/jobs/"+j.ID+"/cancel", map[string]string{}, nil)
	v := waitJob(t, e, j.ID, stateCancelled, stateDone)
	if v.State != stateCancelled {
		t.Errorf("state %s", v.State)
	}
}

func TestSettingsValidation(t *testing.T) {
	e := newEnv(t, false)
	if c, _ := e.do(t, "POST", "/api/settings", Settings{Lang: "xx"}, nil); c != 400 {
		t.Errorf("lang: %d", c)
	}
	if c, _ := e.do(t, "POST", "/api/settings", Settings{Lang: "ja", Theme: "dark", RulesDir: "/no/such/dir"}, nil); c != 400 {
		t.Errorf("dir: %d", c)
	}
	if c, b := e.do(t, "POST", "/api/settings", Settings{Lang: "ja", Theme: "dark"}, nil); c != 200 || !bytes.Contains(b, []byte(`"ja"`)) {
		t.Errorf("save: %d %s", c, b)
	}
}

// The en-US catalog must contain every key the front end references, and every error code.
func TestFrontEndKeysExist(t *testing.T) {
	cat := i18n.Messages("en-US")
	has := func(k string) bool {
		if _, ok := cat[k]; ok {
			return true
		}
		_, ok := cat[k+".other"]
		return ok
	}
	re := regexp.MustCompile(`'(gui\.[a-zA-Z0-9_.]*[a-zA-Z0-9_])'`)
	for _, f := range []string{"assets/app.js"} {
		b, _ := assetsFS.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !has(m[1]) && !strings.HasSuffix(m[1], "_") { // "gui.fix.phase_" + phase: checked below
				t.Errorf("%s uses missing key %s", f, m[1])
			}
		}
	}
	idx, _ := assetsFS.ReadFile("assets/index.html")
	for _, m := range regexp.MustCompile(`data-i18n(?:-aria)?="(gui\.[a-z_.]+)"`).FindAllStringSubmatch(string(idx), -1) {
		if !has(m[1]) {
			t.Errorf("index.html uses missing key %s", m[1])
		}
	}
	for _, c := range errorCodes {
		if !has("gui.err." + c) {
			t.Errorf("error code %s has no gui.err message", c)
		}
	}
	for _, k := range []string{"gui.sev.critical", "gui.sev.high", "gui.sev.medium", "gui.sev.low", "gui.sev.other", "gui.status.error",
		"gui.verdict.good", "gui.verdict.fair", "gui.verdict.poor", "gui.theme.system", "gui.theme.light", "gui.theme.dark",
		"gui.fix.phase_snapshot", "gui.fix.phase_applying", "gui.fix.phase_waiting", "gui.a11y.skip", "gui.auth.required",
		"gui.fx.risk_low", "gui.fx.risk_medium", "gui.fx.risk_high", "gui.diff.err_unreadable", "gui.diff.err_bad_regex",
		"gui.dev.sample_compliant", "gui.dev.sample_noncompliant", "gui.set.theme_system", "gui.set.theme_light", "gui.set.theme_dark",
		"gui.cmp.fixed", "gui.cmp.regressed", "gui.cmp.added", "gui.cmp.removed", "gui.cat.all", "gui.action.device_config"} {
		if !has(k) {
			t.Errorf("missing %s", k)
		}
	}
	// dynamic families: platform categories and groups, risk reasons, export formats
	for c, groups := range categoryGroups {
		if !has("gui.cat." + c) {
			t.Errorf("category %s has no gui.cat message", c)
		}
		for _, g := range groups {
			if !has("gui.group." + g) {
				t.Errorf("group %s has no gui.group message", g)
			}
		}
	}
	for _, r := range []string{"suggestion", "single_line", "verified", "unverified", "database", "command", "service_or_access", "permissions_or_policy", "important_control"} {
		if !has("gui.fx.reason." + r) {
			t.Errorf("risk reason %s has no message", r)
		}
	}
	for _, f := range report.Formats {
		if !has("gui.export." + f) {
			t.Errorf("format %s has no gui.export message", f)
		}
	}
	for k := range cat {
		if strings.HasSuffix(k, ".one") && cat[strings.TrimSuffix(k, ".one")+".other"] == "" {
			t.Errorf("plural %s lacks .other", k)
		}
	}
}

func TestNoExternalURLsInAssets(t *testing.T) {
	for _, f := range []string{"assets/app.js", "assets/index.html", "assets/style.css"} {
		b, _ := assetsFS.ReadFile(f)
		for _, m := range regexp.MustCompile(`(?:src|href)="(https?:[^"]+)"|url\((https?:[^)]+)\)|fetch\('(https?:[^']+)'`).FindAllString(string(b), -1) {
			t.Errorf("%s references an external resource: %s", f, m)
		}
	}
}

func TestCaptureRestoresStdout(t *testing.T) {
	orig := os.Stdout
	out, err := capture(func() error { os.Stdout.WriteString("hello"); return nil })
	if err != nil || out != "hello" || os.Stdout != orig {
		t.Errorf("capture: %q %v", out, err)
	}
}
