// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package server implements the self-hosted TMAegis API and dashboard: it serves the rule
// catalogue and stores scan reports pushed by agents so teams get a fleet-wide view.
package server

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

//go:embed web
var webFS embed.FS

const maxReportBytes = 8 << 20

// Config configures a Server.
type Config struct {
	RulesDir string
	DataDir  string
	// Token is the bearer token required to push or read reports. Reports contain host
	// configuration details, so the report endpoints stay disabled when it is empty.
	Token string
}

// Stored is a persisted report with its metadata.
type Stored struct {
	ID         string             `json:"id"`
	Host       string             `json:"host"`
	ReceivedAt time.Time          `json:"received_at"`
	Report     *engine.ScanReport `json:"report"`
}

type Server struct {
	cfg Config
	mu  sync.Mutex
}

// New validates the configuration and prepares the data directory.
func New(cfg Config) (*Server, error) {
	if cfg.RulesDir == "" || cfg.DataDir == "" {
		return nil, errors.New("rules dir and data dir are required")
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "reports"), 0o700); err != nil {
		return nil, err
	}
	if _, err := rules.LoadFromDir(cfg.RulesDir, "all", 2); err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	return &Server{cfg: cfg}, nil
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/rules", s.listRules)
	mux.HandleFunc("GET /api/rules/{id}", s.getRule)
	mux.HandleFunc("POST /api/reports", s.auth(s.postReport))
	mux.HandleFunc("GET /api/reports", s.auth(s.listReports))
	mux.HandleFunc("GET /api/reports/{id}", s.auth(s.getReport))
	mux.HandleFunc("GET /api/dashboard", s.auth(s.dashboard))
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServerFS(sub))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token == "" {
			writeErr(w, http.StatusForbidden, "report endpoints are disabled: start the server with a token")
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.Token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="tmaegis"`)
			writeErr(w, http.StatusUnauthorized, "invalid or missing bearer token")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ---- rules

func (s *Server) loadRules() ([]rules.Rule, error) {
	return rules.LoadFromDir(s.cfg.RulesDir, "all", 2)
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	all, err := s.loadRules()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	q := r.URL.Query()
	target, text, sev := q.Get("target"), strings.ToLower(q.Get("q")), q.Get("severity")
	out := []rules.Rule{}
	for _, rule := range all {
		if target != "" && !strings.EqualFold(rule.Target, target) {
			continue
		}
		if sev != "" && !strings.EqualFold(rule.Severity, sev) {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(rule.ID+" "+rule.Title+" "+rule.Description), text) {
			continue
		}
		out = append(out, rule)
	}
	writeJSON(w, 200, out)
}

func (s *Server) getRule(w http.ResponseWriter, r *http.Request) {
	all, err := s.loadRules()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	for _, rule := range all {
		if rule.ID == r.PathValue("id") {
			writeJSON(w, 200, rule)
			return
		}
	}
	writeErr(w, 404, "rule not found")
}

// ---- reports

var safeID = regexp.MustCompile(`^[a-z0-9-]{1,80}$`)

func (s *Server) reportPath(id string) string {
	return filepath.Join(s.cfg.DataDir, "reports", id+".json")
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return time.Now().UTC().Format("20060102t150405") + "-" + hex.EncodeToString(b)
}

func sanitizeHost(h string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(h) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	if b.Len() > 100 {
		return b.String()[:100]
	}
	return b.String()
}

func (s *Server) postReport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxReportBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "report too large or unreadable")
		return
	}
	var rep engine.ScanReport
	if err := json.Unmarshal(body, &rep); err != nil || rep.Timestamp.IsZero() {
		writeErr(w, 400, "body must be a TMAegis JSON scan report")
		return
	}
	st := Stored{ID: newID(), Host: sanitizeHost(rep.Hostname), ReceivedAt: time.Now().UTC(), Report: &rep}
	data, _ := json.Marshal(st)
	s.mu.Lock()
	err = os.WriteFile(s.reportPath(st.ID), data, 0o600)
	s.mu.Unlock()
	if err != nil {
		writeErr(w, 500, "could not store report")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": st.ID})
}

func (s *Server) readAll() ([]Stored, error) {
	entries, err := os.ReadDir(filepath.Join(s.cfg.DataDir, "reports"))
	if err != nil {
		return nil, err
	}
	var out []Stored
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "reports", e.Name()))
		if err != nil {
			continue
		}
		var st Stored
		if json.Unmarshal(data, &st) == nil && st.Report != nil {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ReceivedAt.After(out[j].ReceivedAt) })
	return out, nil
}

type summary struct {
	ID         string    `json:"id"`
	Host       string    `json:"host"`
	ReceivedAt time.Time `json:"received_at"`
	Score      float64   `json:"score"`
	Passed     int       `json:"passed"`
	Failed     int       `json:"failed"`
	Total      int       `json:"total"`
}

func summarize(st Stored) summary {
	return summary{st.ID, st.Host, st.ReceivedAt, st.Report.Score(), st.Report.PassedCount, st.Report.FailedCount, st.Report.TotalRules}
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	all, err := s.readAll()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	out := []summary{}
	for _, st := range all {
		out = append(out, summarize(st))
	}
	writeJSON(w, 200, out)
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !safeID.MatchString(id) { // blocks path traversal before touching the filesystem
		writeErr(w, 404, "report not found")
		return
	}
	data, err := os.ReadFile(s.reportPath(id))
	if err != nil {
		writeErr(w, 404, "report not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// dashboard returns the latest report per host plus fleet-wide aggregates.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	all, err := s.readAll()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	latest := map[string]Stored{}
	for _, st := range all { // newest first, so the first seen per host wins
		if _, ok := latest[st.Host]; !ok {
			latest[st.Host] = st
		}
	}
	hosts := []summary{}
	sumScore := 0.0
	failedBySeverity := map[string]int{}
	for _, st := range latest {
		sm := summarize(st)
		hosts = append(hosts, sm)
		sumScore += sm.Score
		for _, res := range st.Report.Results {
			if res.Status == engine.StatusFail || res.Status == engine.StatusErr {
				failedBySeverity[strings.ToLower(res.Rule.Severity)]++
			}
		}
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].Score < hosts[j].Score }) // worst first
	avg := 0.0
	if len(hosts) > 0 {
		avg = sumScore / float64(len(hosts))
	}
	writeJSON(w, 200, map[string]interface{}{
		"hosts": hosts, "host_count": len(hosts), "average_score": avg, "failed_by_severity": failedBySeverity,
	})
}
