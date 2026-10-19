// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package gui is the local web interface behind `tmaegis gui`. It serves a single-page
// app embedded in the binary on 127.0.0.1 only, guarded by a random per-run token, a Host and
// Origin allow-list and a per-run action nonce. It makes no external requests.
package gui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/rules"
)

//go:embed assets
var assetsFS embed.FS

const (
	cookieName  = "tmaegis_gui"
	nonceHeader = "X-TMAegis-Nonce"
)

// AIInfo is shown (read only) on the Settings screen. API keys are never handled by the GUI.
type AIInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	KeyEnv   string `json:"key_env,omitempty"`
}

// Options configures a Server.
type Options struct {
	Port        int           // 0 picks a random free port
	Lang        string        // default UI language when the user has not chosen one
	DataDir     string        // default: os.UserConfigDir()/tmaegis/gui
	RulesDir    string        // default rules directory
	SnapshotDir string        // where snapshots live (shared with `tmaegis apply`)
	Concurrency int           // parallel audits
	DeadmanSec  int           // default dead-man timer for Apply
	Version     string        // shown in About
	OpenTimeout time.Duration // exit when the browser never connects within this time (0 = never)
	IdleTimeout time.Duration // exit after this long without UI activity or running jobs (0 = never)
	OSFamily    string        // default --os-family override
	AI          AIInfo
	Out         io.Writer // status messages (default: discard)

	// Test seams.
	IsElevated func() bool
	DetectHost func() discovery.Host
}

// Server is the local GUI server.
type Server struct {
	opts   Options
	token  string
	nonce  string
	port   int
	store  *store
	jobs   *jobs
	done   chan struct{}
	quitCh chan struct{}

	mu       sync.Mutex
	lastSeen time.Time
	authed   bool
	host     *discovery.Host
	previews map[string]time.Time
	stats    map[string]map[string]*TargetStats // per rules directory
	ruleSets map[string][]rules.Rule            // every rule of a directory, all levels
	renderMu sync.Mutex                         // report.Lang is process-wide
	once     sync.Once
	quitOnce sync.Once
}

// New prepares a server (nothing is listening yet).
func New(o Options) (*Server, error) {
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.Concurrency < 1 {
		o.Concurrency = 4
	}
	if o.DeadmanSec <= 0 {
		o.DeadmanSec = 60
	}
	if o.IsElevated == nil {
		o.IsElevated = isElevated
	}
	if o.DetectHost == nil {
		o.DetectHost = discovery.Detect
	}
	if o.RulesDir == "" {
		o.RulesDir = "./rules"
	}
	if o.SnapshotDir == "" {
		o.SnapshotDir = "./.tmaegis/snapshots"
	}
	st, err := newStore(o.DataDir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		opts: o, store: st, token: randomString(24, tokenAlphabet), nonce: randomString(6, nonceAlphabet),
		done: make(chan struct{}), quitCh: make(chan struct{}), lastSeen: time.Now(),
		previews: map[string]time.Time{}, stats: map[string]map[string]*TargetStats{}, ruleSets: map[string][]rules.Rule{},
	}
	s.jobs = newJobs()
	return s, nil
}

const (
	tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	nonceAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no look-alike characters
)

func randomString(n int, alphabet string) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS random source failing is unrecoverable
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Listen binds 127.0.0.1 on the configured (or a random) port.
func (s *Server) Listen() (net.Listener, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.opts.Port))
	if err != nil {
		return nil, err
	}
	s.port = l.Addr().(*net.TCPAddr).Port
	return l, nil
}

// URL is the address to open, including the one-time-exchangeable token.
func (s *Server) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/?t=%s", s.port, s.token)
}

// Token and Nonce are exposed for tests.
func (s *Server) Token() string { return s.token }
func (s *Server) Nonce() string { return s.nonce }

// Serve runs until ctx ends, Quit is pressed, or the open/idle timeout fires.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(l) }()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	start := time.Now()
	var reason string
loop:
	for {
		select {
		case err := <-errCh:
			if err != nil && err != http.ErrServerClosed {
				return err
			}
			break loop
		case <-ctx.Done():
			reason = "interrupt"
			break loop
		case <-s.quitCh:
			reason = "quit"
			break loop
		case now := <-tick.C:
			s.mu.Lock()
			authed, last := s.authed, s.lastSeen
			s.mu.Unlock()
			if !authed && s.opts.OpenTimeout > 0 && now.Sub(start) > s.opts.OpenTimeout {
				reason = "open-timeout"
				break loop
			}
			if authed && s.opts.IdleTimeout > 0 && now.Sub(last) > s.opts.IdleTimeout && !s.jobs.busy() {
				reason = "idle-timeout"
				break loop
			}
		}
	}
	fmt.Fprintf(s.opts.Out, "[*] Shutting down (%s)\n", reason)
	return s.shutdown(srv)
}

func (s *Server) shutdown(srv *http.Server) error {
	s.once.Do(func() { close(s.done) })
	// Unconfirmed changes are rolled back when the GUI goes away: the operator can no longer confirm.
	s.jobs.rollbackPending("shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return srv.Close()
	}
	return nil
}

// Handler returns the full HTTP handler (used directly by tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	mux.HandleFunc("GET /{path...}", s.serveStatic)
	return s.guard(mux)
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("GET /api/host", s.handleHost)
	mux.HandleFunc("GET /api/rulesdirs", s.handleRulesDirs)
	mux.HandleFunc("GET /api/targets", s.handleTargets)
	mux.HandleFunc("GET /api/i18n/{tag}", s.handleI18n)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleSetSettings)
	mux.HandleFunc("POST /api/quit", s.handleQuit)

	mux.HandleFunc("POST /api/scan", s.handleScan)
	mux.HandleFunc("GET /api/jobs", s.handleJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.handleJobEvents)
	mux.HandleFunc("POST /api/jobs/{id}/cancel", s.handleJobCancel)
	mux.HandleFunc("POST /api/jobs/{id}/confirm", s.handleJobConfirm)
	mux.HandleFunc("POST /api/jobs/{id}/rollback", s.handleJobRollback)

	mux.HandleFunc("GET /api/reports", s.handleReports)
	mux.HandleFunc("GET /api/reports/{id}", s.handleReport)
	mux.HandleFunc("GET /api/report/{file}", s.handleReportFile)

	mux.HandleFunc("POST /api/fix/preview", s.handleFixPreview)
	mux.HandleFunc("POST /api/fix/apply", s.handleFixApply)
	mux.HandleFunc("GET /api/snapshots", s.handleSnapshots)
	mux.HandleFunc("POST /api/snapshots/{id}/restore", s.handleRestore)
	mux.HandleFunc("GET /api/history", s.handleHistory)

	mux.HandleFunc("GET /api/platforms", s.handlePlatforms)
	mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	mux.HandleFunc("POST /api/scan/plan", s.handleScanPlan)
	mux.HandleFunc("GET /api/compare", s.handleCompare)
	mux.HandleFunc("POST /api/device-config", s.handleDeviceConfig)
	mux.HandleFunc("GET /api/samples", s.handleSamples)
}

// ---- guard: Host, token, Origin, nonce ----

func (s *Server) hostAllowed(h string) bool {
	host, port, err := net.SplitHostPort(h)
	if err != nil {
		return false
	}
	if s.port != 0 && port != fmt.Sprint(s.port) {
		return false
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

func (s *Server) tokenOK(r *http.Request) (viaHeader, ok bool) {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return true, subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(a, "Bearer ")), []byte(s.token)) == 1
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return false, subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
	}
	return false, false
}

const csp = "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; " +
	"connect-src 'self'; manifest-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'"

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			writeErr(w, http.StatusForbidden, "bad_host", "unexpected Host header")
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if !strings.HasPrefix(r.URL.Path, "/api/report/") || !strings.HasSuffix(r.URL.Path, ".html") {
			h.Set("Content-Security-Policy", csp) // the HTML report carries its own hash-based policy
		}

		if r.Method == http.MethodGet && r.URL.Path == "/" && r.URL.Query().Get("t") != "" {
			if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.token)) == 1 {
				http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
		}
		viaHeader, ok := s.tokenOK(r)
		if !ok {
			if r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "<!doctype html><meta charset=utf-8><title>TMAegis</title><p>%s</p>",
					htmlEscape(i18n.New(s.defaultLang()).T("gui.auth.required")))
				return
			}
			writeErr(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				u, err := url.Parse(o)
				if err != nil || u.Scheme != "http" || !s.hostAllowed(u.Host) {
					writeErr(w, http.StatusForbidden, "bad_origin", "cross-origin request refused")
					return
				}
			} else if !viaHeader {
				writeErr(w, http.StatusForbidden, "bad_origin", "Origin header required")
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get(nonceHeader)), []byte(s.nonce)) != 1 {
				writeErr(w, http.StatusForbidden, "bad_nonce", "missing or invalid action nonce")
				return
			}
			if r.ContentLength != 0 && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeErr(w, http.StatusUnsupportedMediaType, "bad_content_type", "application/json required")
				return
			}
		}
		s.mu.Lock()
		s.authed = true
		s.lastSeen = time.Now()
		s.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

func (s *Server) touch() {
	s.mu.Lock()
	s.lastSeen = time.Now()
	s.mu.Unlock()
}

// ---- static files ----

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case name == "" || name == "index.html":
		name = "index.html"
	case strings.HasPrefix(name, "static/"):
		name = strings.TrimPrefix(name, "static/")
	default:
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	sub, _ := fs.Sub(assetsFS, "assets")
	b, err := fs.ReadFile(sub, name)
	if err != nil || strings.Contains(name, "..") {
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	switch {
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(name, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
	case strings.HasSuffix(name, ".txt"):
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	if name == spriteName {
		// The logo sprite is large and referenced by every view: cache it. The versioned URL the UI uses
		// changes with the content, so that one is immutable; any other request revalidates by ETag.
		ic := platformIcons()
		etag := `"` + ic.hash + `"`
		w.Header().Set("ETag", etag)
		if r.URL.Query().Get("v") == ic.hash {
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "private, no-cache")
		}
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write(b)
}

// ---- JSON helpers ----

// Error codes returned to the UI; each has a gui.err.<code> message in the en-US catalog.
var errorCodes = []string{
	"bad_host", "unauthorized", "bad_origin", "bad_nonce", "bad_content_type", "not_found", "bad_request",
	"busy", "not_elevated", "bad_confirm", "preview_required", "no_rules", "snapshot_failed", "invalid_dir",
	"rules_load_failed", "unsupported_format", "render_failed", "restore_failed", "internal", "no_remediation",
	"too_large", "no_device_config",
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && err != io.EOF {
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return false
	}
	return true
}

func (s *Server) defaultLang() string {
	if st := s.store.settings(); st.Lang != "" {
		return i18n.Resolve(st.Lang)
	}
	if s.opts.Lang != "" {
		return i18n.Resolve(s.opts.Lang)
	}
	return i18n.Default
}
