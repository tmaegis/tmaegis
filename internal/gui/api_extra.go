// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// loadAll returns every rule of a directory (both levels). Scans pass fresh=true so an
// edited rules directory is always re-read; the read-only views reuse the cached copy.
func (s *Server) loadAll(dir string, fresh bool) ([]rules.Rule, error) {
	s.mu.Lock()
	all, ok := s.ruleSets[dir]
	s.mu.Unlock()
	if ok && !fresh {
		return all, nil
	}
	all, err := rules.LoadFiltered(dir, rules.Filter{MaxLevel: 2})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ruleSets[dir] = all
	if fresh {
		delete(s.stats, dir) // recomputed from the fresh copy on next use
	}
	s.mu.Unlock()
	return all, nil
}

// ---- scan plan: what a scan would run, without running it ----

type planTarget struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Count    int    `json:"count"`
}

func (s *Server) handleScanPlan(w http.ResponseWriter, r *http.Request) {
	var p ScanParams
	if !decode(w, r, &p) {
		return
	}
	if p.Scope == "" {
		p.Scope = "auto"
	}
	if p.Level != 2 {
		p.Level = 1
	}
	if p.RulesDir == "" {
		p.RulesDir = s.rulesDir()
	}
	if p.OSFamily == "" {
		p.OSFamily = s.opts.OSFamily
	}
	if !isDir(p.RulesDir) {
		writeErr(w, http.StatusBadRequest, "invalid_dir", "rules directory not found: "+p.RulesDir)
		return
	}
	all, err := s.loadAll(p.RulesDir, false)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "rules_load_failed", err.Error())
		return
	}
	list, err := selectFrom(all, p, s.detectHost(false))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	byTarget := map[string]int{}
	sev := map[string]int{}
	manual, device, fixable, l2 := 0, 0, 0, 0
	for _, r := range list {
		byTarget[strings.ToLower(r.Target)]++
		sev[strings.ToLower(r.Severity)]++
		switch r.Audit.Type {
		case "manual":
			manual++
		case "config_regex":
			device++
		}
		if r.Remediation.Type != "" {
			fixable++
		}
		if r.Level == 2 {
			l2++
		}
	}
	targets := []planTarget{}
	for id, n := range byTarget {
		m := metaOf(id)
		targets = append(targets, planTarget{id, m.name, m.category, n})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Count > targets[j].Count || (targets[i].Count == targets[j].Count && targets[i].ID < targets[j].ID)
	})
	// Device rules that a supplied configuration would enable, so the wizard can say so.
	deviceAvailable := 0
	if p.DeviceConfig == "" && p.Scope == "target" {
		q := p
		q.DeviceConfig = "x"
		if more, err := selectFrom(all, q, s.detectHost(false)); err == nil {
			deviceAvailable = len(more)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total": len(list), "targets": targets, "severity": sev, "manual": manual, "device": device,
		"auto": len(list) - manual - device, "fixable": fixable, "level2": l2, "level": p.Level,
		"device_available": deviceAvailable, "scope": p.Scope, "rules_dir": p.RulesDir,
	})
}

// ---- dashboard ----

type catFail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Failed   int    `json:"failed"`
	Total    int    `json:"total"`
}

type sevRow struct {
	Pass int `json:"pass"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

func sevKey(s string) string {
	s = strings.ToLower(s)
	switch s {
	case "critical", "high", "medium", "low":
		return s
	}
	return "other"
}

func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	reps := s.store.reports()
	host := s.detectHost(false)
	out := map[string]any{"elevated": s.opts.IsElevated(), "reports": len(reps), "snapshots": len(s.snapshots())}

	// detected benchmarks with their rule counts
	type bench struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Rules    int    `json:"rules"`
		Level1   int    `json:"level1"`
		Auto     int    `json:"auto"`
	}
	benches := []bench{}
	rulesTotal := 0
	if st, err := s.targetStats(s.rulesDir()); err == nil {
		for _, t := range host.Targets() {
			if ts, ok := st[t]; ok {
				benches = append(benches, bench{t, ts.Name, ts.Category, ts.Total, ts.Level1, ts.Auto})
			} else {
				m := metaOf(t)
				benches = append(benches, bench{ID: t, Name: m.name, Category: m.category})
			}
		}
		for _, t := range st {
			rulesTotal += t.Total
		}
		out["catalog"] = map[string]int{"rules": rulesTotal, "targets": len(st)}
	} else {
		out["catalog_error"] = err.Error()
	}
	if host.Software == nil {
		host.Software = []string{}
	}
	out["host"] = host
	out["benchmarks"] = benches

	trend := []map[string]any{}
	for i := len(reps) - 1; i >= 0; i-- {
		trend = append(trend, map[string]any{"id": reps[i].ID, "time": reps[i].Created.UTC().Format(time.RFC3339), "score": reps[i].Score})
	}
	if len(trend) > 30 {
		trend = trend[len(trend)-30:]
	}
	out["trend"] = trend
	if len(reps) == 0 {
		writeJSON(w, http.StatusOK, out)
		return
	}
	last := reps[0]
	out["last"] = last
	if len(reps) > 1 {
		out["previous"] = reps[1]
	}
	sr, err := s.store.report(last.ID)
	if err != nil || sr.Report == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	sev := map[string]*sevRow{}
	for _, k := range []string{"critical", "high", "medium", "low", "other"} {
		sev[k] = &sevRow{}
	}
	cats := map[string]*catFail{}
	manual, fixable := 0, 0
	for _, res := range sr.Report.Results {
		k := sevKey(res.Rule.Severity)
		id := strings.ToLower(res.Rule.Target)
		c := cats[id]
		if c == nil {
			m := metaOf(id)
			c = &catFail{ID: id, Name: m.name, Category: m.category}
			cats[id] = c
		}
		c.Total++
		switch res.Status {
		case engine.StatusPass:
			sev[k].Pass++
		case engine.StatusFail, engine.StatusErr:
			sev[k].Fail++
			c.Failed++
			if res.Rule.Remediation.Type != "" {
				fixable++
			}
		default:
			sev[k].Skip++
			if res.Rule.Audit.Type == "manual" {
				manual++
			}
		}
	}
	top := []*catFail{}
	for _, c := range cats {
		if c.Failed > 0 {
			top = append(top, c)
		}
	}
	sort.Slice(top, func(i, j int) bool {
		return top[i].Failed > top[j].Failed || (top[i].Failed == top[j].Failed && top[i].ID < top[j].ID)
	})
	if len(top) > 6 {
		top = top[:6]
	}
	out["severity"] = sev
	out["top_failing"] = top
	out["manual"] = manual
	out["fixable"] = fixable
	writeJSON(w, http.StatusOK, out)
}

// ---- compare two scans ----

type cmpRow struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Target   string `json:"target"`
	Severity string `json:"severity"`
	Before   string `json:"before,omitempty"`
	After    string `json:"after,omitempty"`
}

const compareLimit = 500

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	a, errA := s.store.report(r.URL.Query().Get("a"))
	b, errB := s.store.report(r.URL.Query().Get("b"))
	if errA != nil || errB != nil || a.Report == nil || b.Report == nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	if a.Created.After(b.Created) { // always compare older (before) with newer (after)
		a, b = b, a
	}
	failing := func(st engine.Status) bool { return st == engine.StatusFail || st == engine.StatusErr }
	before := map[string]engine.RuleResult{}
	for _, res := range a.Report.Results {
		before[res.Rule.ID] = res
	}
	lists := map[string][]cmpRow{"fixed": {}, "regressed": {}, "added": {}, "removed": {}}
	counts := map[string]int{"fixed": 0, "regressed": 0, "added": 0, "removed": 0, "unchanged": 0}
	add := func(kind string, row cmpRow) {
		counts[kind]++
		if len(lists[kind]) < compareLimit {
			lists[kind] = append(lists[kind], row)
		}
	}
	seen := map[string]bool{}
	for _, res := range b.Report.Results {
		seen[res.Rule.ID] = true
		row := cmpRow{ID: res.Rule.ID, Title: res.Rule.Title, Target: res.Rule.Target, Severity: res.Rule.Severity, After: string(res.Status)}
		old, ok := before[res.Rule.ID]
		switch {
		case !ok:
			add("added", row)
		case failing(old.Status) && res.Status == engine.StatusPass:
			row.Before = string(old.Status)
			add("fixed", row)
		case old.Status == engine.StatusPass && failing(res.Status):
			row.Before = string(old.Status)
			add("regressed", row)
		default:
			counts["unchanged"]++
		}
	}
	for _, res := range a.Report.Results {
		if !seen[res.Rule.ID] {
			add("removed", cmpRow{ID: res.Rule.ID, Title: res.Rule.Title, Target: res.Rule.Target, Severity: res.Rule.Severity, Before: string(res.Status)})
		}
	}
	for _, l := range lists {
		sort.Slice(l, func(i, j int) bool { return l[i].ID < l[j].ID })
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"before": summarize(a), "after": summarize(b), "delta": b.Score - a.Score,
		"counts": counts, "lists": lists, "limit": compareLimit,
	})
}

// ---- device configurations (the GUI equivalent of --device-config) ----

const maxDeviceConfig = 8 << 20

var devRe = regexp.MustCompile(`^dev-[0-9a-f]{1,32}$`)

type deviceReq struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Sample  string `json:"sample,omitempty"`  // a target from samples/manifest.json
	Variant string `json:"variant,omitempty"` // compliant or noncompliant
}

// deviceConfigPath returns the stored file of an uploaded configuration, or "" when unknown.
func (st *store) deviceConfigPath(id string) string {
	if !devRe.MatchString(id) {
		return ""
	}
	p := filepath.Join(st.dir, "devices", id+".txt")
	if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return p
}

func (st *store) saveDeviceConfig(content []byte) (string, error) {
	if err := os.MkdirAll(filepath.Join(st.dir, "devices"), 0o700); err != nil {
		return "", err
	}
	id := fmt.Sprintf("dev-%x", time.Now().UTC().UnixNano())
	st.mu.Lock()
	defer st.mu.Unlock()
	return id, writeFileAtomic(filepath.Join(st.dir, "devices", id+".txt"), content)
}

func (s *Server) handleDeviceConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxDeviceConfig*2) // JSON escaping can double the size
	var req deviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "the configuration file is too large")
			return
		}
		writeErr(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	content := []byte(req.Content)
	name := filepath.Base(strings.TrimSpace(req.Name))
	if req.Sample != "" {
		smp, dir := s.samples()
		i := slices.IndexFunc(smp, func(x sampleInfo) bool { return x.Target == req.Sample })
		if i < 0 {
			writeErr(w, http.StatusNotFound, "not_found", "no such sample")
			return
		}
		rel := smp[i].Noncompliant
		if req.Variant == "compliant" {
			rel = smp[i].Compliant
		}
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			writeErr(w, http.StatusNotFound, "not_found", "sample file not found")
			return
		}
		content, name = b, filepath.Base(rel)
	}
	if len(content) == 0 {
		writeErr(w, http.StatusBadRequest, "bad_request", "the configuration is empty")
		return
	}
	if len(content) > maxDeviceConfig {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "the configuration file is too large")
		return
	}
	if name == "" || name == "." {
		name = "device-config.txt"
	}
	id, err := s.store.saveDeviceConfig(content)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.store.log("device_config", name, true)
	lines := strings.Count(string(content), "\n")
	if !strings.HasSuffix(string(content), "\n") {
		lines++
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": name, "size": len(content), "lines": lines})
}

type sampleInfo struct {
	Target       string `json:"target"`
	Compliant    string `json:"compliant"`
	Noncompliant string `json:"noncompliant"`
	Rules        int    `json:"rules"`
}

// samples finds the synthetic device configurations shipped in the repository (samples/),
// next to the rules directory or in the working directory.
func (s *Server) samples() ([]sampleInfo, string) {
	cands := []string{"samples"}
	if abs, err := filepath.Abs(s.rulesDir()); err == nil {
		cands = append([]string{filepath.Join(filepath.Dir(abs), "samples")}, cands...)
	}
	for _, dir := range cands {
		b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		var list []sampleInfo
		if json.Unmarshal(b, &list) == nil {
			return list, dir
		}
	}
	return []sampleInfo{}, ""
}

func (s *Server) handleSamples(w http.ResponseWriter, _ *http.Request) {
	list, dir := s.samples()
	writeJSON(w, http.StatusOK, map[string]any{"samples": list, "found": dir != ""})
}
