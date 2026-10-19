// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

func TestRedactSensitiveData(t *testing.T) {
	in := "host 10.0.1.55 password=hunter2 token: abc123 API_KEY=zzz fine text"
	out := RedactSensitiveData(in)
	for _, leak := range []string{"10.0.1.55", "hunter2", "abc123", "zzz"} {
		if strings.Contains(out, leak) {
			t.Errorf("%q leaked in %q", leak, out)
		}
	}
	if !strings.Contains(out, "fine text") {
		t.Errorf("non-sensitive text was damaged: %q", out)
	}
}

func TestNewAnalyzerValidation(t *testing.T) {
	if _, err := NewAnalyzer(Config{Provider: "openai"}); err == nil {
		t.Error("cloud provider without key must error")
	}
	if _, err := NewAnalyzer(Config{Provider: "bogus"}); err == nil {
		t.Error("unknown provider must error")
	}
	a, err := NewAnalyzer(Config{})
	if err != nil || !strings.HasPrefix(a.Name(), "OLLAMA") {
		t.Errorf("default should be ollama, got %v %v", a, err)
	}
}

func TestOllamaAnalysisIsRedacted(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		body = string(buf[:n])
		w.Write([]byte(`{"response":"ok-analysis"}`))
	}))
	defer srv.Close()

	a, _ := NewAnalyzer(Config{Provider: "ollama", Endpoint: srv.URL})
	rep := &engine.ScanReport{OS: "linux", Results: []engine.RuleResult{{
		Rule: rules.Rule{ID: "R1", Title: "t"}, Status: engine.StatusFail, Details: "server 192.168.1.9 password=s3cret",
	}}}
	out, err := a.AnalyzeScanReport(context.Background(), rep)
	if err != nil || out != "ok-analysis" {
		t.Fatalf("got %q, %v", out, err)
	}
	if strings.Contains(body, "192.168.1.9") || strings.Contains(body, "s3cret") {
		t.Fatalf("sensitive data reached the LLM request: %s", body)
	}

	pass := &engine.ScanReport{Results: []engine.RuleResult{{Status: engine.StatusPass}}}
	if out, _ := a.AnalyzeScanReport(context.Background(), pass); !strings.Contains(out, "passed") {
		t.Errorf("all-pass report should short-circuit, got %q", out)
	}
}

func TestHostContextIsIncludedAndRedacted(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		body = string(buf[:n])
		w.Write([]byte(`{"response":"ok"}`))
	}))
	defer srv.Close()
	a, _ := NewAnalyzer(Config{Provider: "ollama", Endpoint: srv.URL, HostContext: "listening 10.1.2.3:5432 postgres"})
	rep := &engine.ScanReport{OS: "linux", Results: []engine.RuleResult{{Rule: rules.Rule{ID: "R"}, Status: engine.StatusFail}}}
	if _, err := a.AnalyzeScanReport(context.Background(), rep); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "postgres") || strings.Contains(body, "10.1.2.3") {
		t.Fatalf("host context missing or unredacted: %s", body)
	}
}
