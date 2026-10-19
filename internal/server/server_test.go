// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

const token = "s3cret-token"

func newServer(t *testing.T, tok string) *httptest.Server {
	t.Helper()
	s, err := New(Config{RulesDir: "../../rules", DataDir: t.TempDir(), Token: tok})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func do(t *testing.T, method, url, tok string, body []byte) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decode(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func report(host string, passed, failed int, sev string) []byte {
	rep := engine.ScanReport{Timestamp: time.Now(), Hostname: host, TotalRules: passed + failed, PassedCount: passed, FailedCount: failed,
		Results: []engine.RuleResult{{Rule: rules.Rule{ID: "R1", Title: "t", Severity: sev}, Status: engine.StatusFail}}}
	b, _ := json.Marshal(rep)
	return b
}

func TestRulesAreFilterableAndPublic(t *testing.T) {
	ts := newServer(t, token)
	var all, nginx []rules.Rule
	decode(t, do(t, "GET", ts.URL+"/api/rules", "", nil), &all)
	decode(t, do(t, "GET", ts.URL+"/api/rules?target=nginx", "", nil), &nginx)
	if len(all) < 10 || len(nginx) == 0 || len(nginx) >= len(all) {
		t.Fatalf("filtering broken: %d total, %d nginx", len(all), len(nginx))
	}
	var one rules.Rule
	resp := do(t, "GET", ts.URL+"/api/rules/"+nginx[0].ID, "", nil)
	decode(t, resp, &one)
	if one.ID != nginx[0].ID {
		t.Fatal("get by id")
	}
	if do(t, "GET", ts.URL+"/api/rules/NOPE", "", nil).StatusCode != 404 {
		t.Fatal("unknown rule must 404")
	}
	var q []rules.Rule
	decode(t, do(t, "GET", ts.URL+"/api/rules?q=server_tokens", "", nil), &q)
	if len(q) == 0 {
		t.Fatal("text search")
	}
}

func TestReportEndpointsRequireToken(t *testing.T) {
	ts := newServer(t, token)
	for _, c := range []struct{ method, path string }{{"POST", "/api/reports"}, {"GET", "/api/reports"}, {"GET", "/api/dashboard"}, {"GET", "/api/reports/x"}} {
		if got := do(t, c.method, ts.URL+c.path, "", nil).StatusCode; got != 401 {
			t.Errorf("%s %s without token = %d", c.method, c.path, got)
		}
		if got := do(t, c.method, ts.URL+c.path, "wrong", nil).StatusCode; got != 401 {
			t.Errorf("%s %s wrong token = %d", c.method, c.path, got)
		}
	}
	off := newServer(t, "")
	if got := do(t, "POST", off.URL+"/api/reports", "anything", report("h", 1, 0, "low")).StatusCode; got != 403 {
		t.Fatalf("server without token must disable reports, got %d", got)
	}
}

func TestPushListAndDashboard(t *testing.T) {
	ts := newServer(t, token)
	push := func(host string, p, f int, sev string) string {
		resp := do(t, "POST", ts.URL+"/api/reports", token, report(host, p, f, sev))
		if resp.StatusCode != 201 {
			t.Fatalf("push = %d", resp.StatusCode)
		}
		var out map[string]string
		decode(t, resp, &out)
		return out["id"]
	}
	push("web-01", 5, 5, "critical") // superseded by the newer report below
	time.Sleep(1100 * time.Millisecond)
	id := push("web-01", 9, 1, "high")
	push("db-01", 1, 3, "critical")

	var list []summary
	decode(t, do(t, "GET", ts.URL+"/api/reports", token, nil), &list)
	if len(list) != 3 || list[0].ReceivedAt.Before(list[2].ReceivedAt) {
		t.Fatalf("list: %+v", list)
	}

	var d struct {
		Hosts []summary      `json:"hosts"`
		Count int            `json:"host_count"`
		Avg   float64        `json:"average_score"`
		BySev map[string]int `json:"failed_by_severity"`
	}
	decode(t, do(t, "GET", ts.URL+"/api/dashboard", token, nil), &d)
	if d.Count != 2 || d.Hosts[0].Host != "db-01" || d.Hosts[0].Score != 25 || d.Avg != 57.5 {
		t.Fatalf("dashboard should use latest report per host, worst first: %+v", d)
	}
	if d.BySev["high"] != 1 || d.BySev["critical"] != 1 {
		t.Fatalf("severity counts must come from latest reports only: %v", d.BySev)
	}

	var st Stored
	decode(t, do(t, "GET", ts.URL+"/api/reports/"+id, token, nil), &st)
	if st.Host != "web-01" || st.Report.PassedCount != 9 {
		t.Fatalf("stored report: %+v", st)
	}
}

func TestInputValidation(t *testing.T) {
	ts := newServer(t, token)
	if got := do(t, "POST", ts.URL+"/api/reports", token, []byte("{not json")).StatusCode; got != 400 {
		t.Errorf("bad json = %d", got)
	}
	if got := do(t, "POST", ts.URL+"/api/reports", token, []byte(`{"total_rules":1}`)).StatusCode; got != 400 {
		t.Errorf("missing timestamp = %d", got)
	}
	big := append([]byte(`{"timestamp":"2026-01-01T00:00:00Z","hostname":"`), bytes.Repeat([]byte("a"), maxReportBytes)...)
	if got := do(t, "POST", ts.URL+"/api/reports", token, big).StatusCode; got != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized = %d", got)
	}
	for _, id := range []string{"..%2F..%2Fetc%2Fpasswd", "UPPER", "a.b"} {
		if got := do(t, "GET", ts.URL+"/api/reports/"+id, token, nil).StatusCode; got != 404 {
			t.Errorf("traversal id %q = %d", id, got)
		}
	}
	if sanitizeHost("../Evil Host<script>") != "..evilhostscript" {
		t.Errorf("sanitizeHost: %q", sanitizeHost("../Evil Host<script>"))
	}
	if sanitizeHost("") != "unknown" {
		t.Error("empty host")
	}
}

func TestStaticDashboardAndHeaders(t *testing.T) {
	ts := newServer(t, token)
	resp := do(t, "GET", ts.URL+"/", "", nil)
	var b bytes.Buffer
	b.ReadFrom(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(b.String(), "TMAegis Security Center") {
		t.Fatalf("dashboard page: %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'self'") || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers missing")
	}
	if strings.Contains(b.String(), "<script>") {
		t.Fatal("CSP forbids inline scripts")
	}
	if do(t, "GET", ts.URL+"/app.js", "", nil).StatusCode != 200 {
		t.Fatal("app.js")
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("empty config must error")
	}
}
