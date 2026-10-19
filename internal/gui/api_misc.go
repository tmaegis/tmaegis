// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/report"
)

type langInfo struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

func languages() []langInfo {
	out := make([]langInfo, 0, len(i18n.Tags))
	for _, t := range i18n.Tags {
		out = append(out, langInfo{t, i18n.Label(t)})
	}
	return out
}

func (s *Server) handleSession(w http.ResponseWriter, _ *http.Request) {
	exe, _ := os.Executable()
	snapDir, _ := filepath.Abs(s.opts.SnapshotDir)
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.opts.Version, "os": runtime.GOOS, "arch": runtime.GOARCH,
		"elevated": s.opts.IsElevated(), "nonce": s.nonce, "lang": s.defaultLang(),
		"languages": languages(), "settings": s.effectiveSettings(), "exe": exe,
		"snapshot_dir": snapDir, "data_dir": s.store.dir, "ai": s.opts.AI,
		"deadman_sec": s.opts.DeadmanSec, "default_rules_dir": s.opts.RulesDir,
		"formats": report.Formats, "hostname": hostname(), "beta": betaTags(), "icons": platformIcons(),
	})
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

// betaTags lists the machine-translated locales the language picker marks as Beta.
func betaTags() []string {
	out := []string{}
	for _, t := range i18n.Tags {
		if i18n.IsBeta(t) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) effectiveSettings() Settings {
	st := s.store.settings()
	st.Lang = s.defaultLang()
	if st.RulesDir == "" {
		st.RulesDir = s.opts.RulesDir
	}
	return st
}

func (s *Server) detectHost(refresh bool) discovery.Host {
	s.mu.Lock()
	if s.host != nil && !refresh {
		h := *s.host
		s.mu.Unlock()
		return h
	}
	s.mu.Unlock()
	h := s.opts.DetectHost() // may spawn PowerShell on Windows: keep it outside the lock
	s.mu.Lock()
	s.host = &h
	s.mu.Unlock()
	return h
}

func (s *Server) handleHost(w http.ResponseWriter, r *http.Request) {
	h := s.detectHost(r.URL.Query().Get("refresh") == "1")
	if h.Software == nil {
		h.Software = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"host": h, "targets": h.Targets(), "elevated": s.opts.IsElevated()})
}

type dirInfo struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"` // rules, staging, custom
	Exists  bool   `json:"exists"`
	Current bool   `json:"current,omitempty"`
}

func (s *Server) handleRulesDirs(w http.ResponseWriter, _ *http.Request) {
	cur := s.rulesDir()
	seen := map[string]bool{}
	out := []dirInfo{}
	add := func(p, kind string) {
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, dirInfo{Path: p, Kind: kind, Exists: isDir(p), Current: p == cur})
	}
	add(cur, "rules")
	if cur != s.opts.RulesDir {
		add(s.opts.RulesDir, "rules")
	}
	add(filepath.Join(filepath.Dir(filepath.Clean(cur)), "rules-staging"), "staging")
	add("rules-staging", "staging")
	writeJSON(w, http.StatusOK, map[string]any{"dirs": out})
}

type targetInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("rules_dir")
	if dir == "" {
		dir = s.rulesDir()
	}
	if !isDir(dir) {
		writeErr(w, http.StatusBadRequest, "invalid_dir", "rules directory not found: "+dir)
		return
	}
	st, err := s.targetStats(dir)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "rules_load_failed", err.Error())
		return
	}
	out := make([]targetInfo, 0, len(st))
	for _, t := range st {
		out = append(out, targetInfo{t.ID, t.Total})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, map[string]any{"targets": out})
}

func (s *Server) handleI18n(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if !slices.Contains(i18n.Tags, tag) {
		writeErr(w, http.StatusNotFound, "not_found", "unsupported language")
		return
	}
	msgs := map[string]string{}
	for k, v := range i18n.Messages(tag) {
		if strings.HasPrefix(k, "gui.") {
			msgs[k] = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tag": tag, "name": i18n.Label(tag), "messages": msgs})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.effectiveSettings())
}

func (s *Server) handleSetSettings(w http.ResponseWriter, r *http.Request) {
	var in Settings
	if !decode(w, r, &in) {
		return
	}
	if in.Lang != "" && !slices.Contains(i18n.Tags, in.Lang) {
		writeErr(w, http.StatusBadRequest, "bad_request", "unsupported language")
		return
	}
	if in.Theme != "light" && in.Theme != "dark" {
		in.Theme = "system"
	}
	if in.RulesDir != "" && !isDir(in.RulesDir) {
		writeErr(w, http.StatusBadRequest, "invalid_dir", "rules directory not found: "+in.RulesDir)
		return
	}
	if in.RulesDir == s.opts.RulesDir {
		in.RulesDir = ""
	}
	if err := s.store.saveSettings(in); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.store.log("settings", "lang="+in.Lang+" theme="+in.Theme+" rules_dir="+in.RulesDir, true)
	writeJSON(w, http.StatusOK, s.effectiveSettings())
}

func (s *Server) handleQuit(w http.ResponseWriter, _ *http.Request) {
	s.store.log("quit", "", true)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	s.quitOnce.Do(func() { close(s.quitCh) })
}
