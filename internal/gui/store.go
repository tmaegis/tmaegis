// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// Settings are the user's GUI preferences. They never contain secrets.
type Settings struct {
	Lang     string `json:"lang"`
	Theme    string `json:"theme"` // system, light, dark
	RulesDir string `json:"rules_dir"`
}

// ScanParams describes one scan request.
type ScanParams struct {
	Scope         string `json:"scope"` // auto, target, dir
	Target        string `json:"target,omitempty"`
	RulesDir      string `json:"rules_dir,omitempty"`
	Level         int    `json:"level"`
	IncludeManual bool   `json:"include_manual"`
	OSFamily      string `json:"os_family,omitempty"`
	// DeviceConfig is the id of an uploaded device configuration (the GUI equivalent of
	// --device-config); DeviceName is the file name the operator chose, for display.
	DeviceConfig string `json:"device_config,omitempty"`
	DeviceName   string `json:"device_name,omitempty"`
}

// StoredReport is a finished scan kept in the per-user data directory.
type StoredReport struct {
	ID      string             `json:"id"`
	Created time.Time          `json:"created"`
	Params  ScanParams         `json:"params"`
	Score   float64            `json:"score"`
	Report  *engine.ScanReport `json:"report"`
}

// ReportSummary is the history row of a stored report.
type ReportSummary struct {
	ID       string     `json:"id"`
	Created  time.Time  `json:"created"`
	Score    float64    `json:"score"`
	Total    int        `json:"total"`
	Passed   int        `json:"passed"`
	Failed   int        `json:"failed"`
	Skipped  int        `json:"skipped"`
	Hostname string     `json:"hostname,omitempty"`
	OS       string     `json:"os"`
	Params   ScanParams `json:"params"`
}

// Action is one line of the audit trail of what the GUI did.
type Action struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Detail string    `json:"detail,omitempty"`
	OK     bool      `json:"ok"`
}

var idRe = regexp.MustCompile(`^scan-[0-9a-f]{1,32}$`)

type store struct {
	dir string
	mu  sync.Mutex
}

func newStore(dir string) (*store, error) {
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("locating the per-user data directory: %w", err)
		}
		dir = filepath.Join(base, "tmaegis", "gui")
	}
	if err := os.MkdirAll(filepath.Join(dir, "reports"), 0o700); err != nil {
		return nil, err
	}
	return &store{dir: dir}, nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func (st *store) settings() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	s := Settings{Theme: "system"}
	if b, err := os.ReadFile(filepath.Join(st.dir, "settings.json")); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Theme != "light" && s.Theme != "dark" {
		s.Theme = "system"
	}
	return s
}

func (st *store) saveSettings(s Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	return writeFileAtomic(filepath.Join(st.dir, "settings.json"), b)
}

func (st *store) saveReport(sr *StoredReport) error {
	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return writeFileAtomic(filepath.Join(st.dir, "reports", sr.ID+".json"), b)
}

func (st *store) report(id string) (*StoredReport, error) {
	if !idRe.MatchString(id) {
		return nil, os.ErrNotExist
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	b, err := os.ReadFile(filepath.Join(st.dir, "reports", id+".json"))
	if err != nil {
		return nil, err
	}
	var sr StoredReport
	if err := json.Unmarshal(b, &sr); err != nil {
		return nil, err
	}
	return &sr, nil
}

func summarize(sr *StoredReport) ReportSummary {
	rep := sr.Report
	sum := ReportSummary{ID: sr.ID, Created: sr.Created, Score: sr.Score, Total: rep.TotalRules,
		Passed: rep.PassedCount, Failed: rep.FailedCount, Hostname: rep.Hostname, OS: rep.OS, Params: sr.Params}
	for _, r := range rep.Results {
		if r.Status == engine.StatusSkip {
			sum.Skipped++
		}
	}
	return sum
}

// reports lists stored reports, newest first. Unreadable files are skipped.
func (st *store) reports() []ReportSummary {
	entries, _ := os.ReadDir(filepath.Join(st.dir, "reports"))
	out := []ReportSummary{}
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".json")
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || !idRe.MatchString(id) {
			continue
		}
		if sr, err := st.report(id); err == nil && sr.Report != nil {
			out = append(out, summarize(sr))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

func (st *store) log(action, detail string, ok bool) {
	b, _ := json.Marshal(Action{Time: time.Now().UTC(), Action: action, Detail: detail, OK: ok})
	st.mu.Lock()
	defer st.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(st.dir, "actions.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// actions returns the newest n audit-trail entries, newest first.
func (st *store) actions(n int) []Action {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := []Action{}
	f, err := os.Open(filepath.Join(st.dir, "actions.jsonl"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		var a Action
		if json.Unmarshal(sc.Bytes(), &a) == nil {
			out = append(out, a)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func newReportID() string { return fmt.Sprintf("scan-%x", time.Now().UTC().UnixNano()) }
