// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/snapshot"
)

// The Apply flow mirrors cmd/apply.go (snapshot, remediate, auto-rollback on error, dead-man
// switch). It is duplicated here rather than shared so the CLI files stay untouched.

const (
	previewTTL    = 30 * time.Minute
	applyTimeout  = 10 * time.Minute
	minDeadmanSec = 15
	maxDeadmanSec = 900
)

var snapRe = regexp.MustCompile(`^snap-[0-9]+$`)

type fixReq struct {
	ReportID    string   `json:"report_id"`
	RuleIDs     []string `json:"rule_ids"`
	ConfirmCode string   `json:"confirm_code"`
	DeadmanSec  int      `json:"deadman_sec"`
}

// FixItem describes what one remediation will do.
type FixItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Severity string   `json:"severity"`
	Type     string   `json:"type"`
	Files    []string `json:"files,omitempty"`
	Command  string   `json:"command,omitempty"`
	Line     string   `json:"line,omitempty"`
	Verify   string   `json:"verify,omitempty"`
	Applied  bool     `json:"applied_directly"` // false for iac_patch: only a suggestion is produced
	Risk     string   `json:"risk"`             // low, medium, high (see riskOf)
	Reasons  []string `json:"risk_reasons,omitempty"`
	Diff     *Diff    `json:"diff,omitempty"` // file_line: the change to the file, computed in preview
}

// pickFixes resolves the requested rule IDs against the failing results of a stored report.
func (s *Server) pickFixes(reportID string, ids []string) (todo []rules.Rule, items []FixItem, unsupported []string, err error) {
	sr, err := s.store.report(reportID)
	if err != nil {
		return nil, nil, nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, res := range sr.Report.Results {
		if !want[res.Rule.ID] || (res.Status != engine.StatusFail && res.Status != engine.StatusErr) {
			continue
		}
		r := res.Rule
		if r.Remediation.Type == "" {
			unsupported = append(unsupported, r.ID)
			continue
		}
		rem := r.Remediation
		files := append([]string(nil), rem.BackupFiles...)
		if rem.FilePath != "" {
			files = append(files, rem.FilePath)
		}
		todo = append(todo, r)
		items = append(items, FixItem{ID: r.ID, Title: r.Title, Severity: r.Severity, Type: rem.Type, Files: dedupe(files),
			Command: rem.Command, Line: strings.TrimSpace(rem.ReplaceLine), Verify: rem.VerifyCmd, Applied: rem.Type != "iac_patch"})
		items[len(items)-1].Risk, items[len(items)-1].Reasons = riskOf(r)
	}
	return todo, items, unsupported, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func previewKey(reportID string, ids []string) string {
	c := append([]string(nil), ids...)
	sort.Strings(c)
	h := sha256.Sum256([]byte(reportID + "\x00" + strings.Join(c, "\x00")))
	return hex.EncodeToString(h[:8])
}

func (s *Server) handleFixPreview(w http.ResponseWriter, r *http.Request) {
	var req fixReq
	if !decode(w, r, &req) {
		return
	}
	todo, items, unsupported, err := s.pickFixes(req.ReportID, req.RuleIDs)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	if len(todo) == 0 {
		writeErr(w, http.StatusBadRequest, "no_remediation", "none of the selected rules can be remediated")
		return
	}
	runner := engine.NewRunnerWithConcurrency(1)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	var out strings.Builder
	var failures []string
	for _, rule := range todo { // per rule, so one failing dry run does not hide the others
		text, err := capture(func() error { return runner.RunRemediation(ctx, []rules.Rule{rule}, true) })
		out.WriteString(text)
		if err != nil {
			failures = append(failures, err.Error())
			fmt.Fprintf(&out, "    [!] %v\n", err)
		}
	}
	s.mu.Lock()
	s.previews[previewKey(req.ReportID, req.RuleIDs)] = time.Now()
	s.mu.Unlock()

	for i, rule := range todo {
		if rule.Remediation.Type == "file_line" {
			items[i].Diff = fileLineDiff(rule.Remediation)
		}
	}
	var backup []string
	for _, it := range items {
		backup = append(backup, it.Files...)
	}
	s.store.log("fix.preview", fmt.Sprintf("%s rules=%d", req.ReportID, len(todo)), len(failures) == 0)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "unsupported": unsupported, "backup_files": dedupe(backup),
		"output": out.String(), "failures": failures, "deadman_sec": s.deadman(0), "snapshot_dir": s.opts.SnapshotDir,
	})
}

func (s *Server) deadman(req int) int {
	if req <= 0 {
		req = s.opts.DeadmanSec
	}
	return min(max(req, minDeadmanSec), maxDeadmanSec)
}

func (s *Server) handleFixApply(w http.ResponseWriter, r *http.Request) {
	var req fixReq
	if !decode(w, r, &req) {
		return
	}
	if !s.opts.IsElevated() {
		s.store.log("fix.apply", "refused: not elevated", false)
		writeErr(w, http.StatusForbidden, "not_elevated", "administrator/root privileges are required to apply changes")
		return
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(req.ConfirmCode))), []byte(s.nonce)) != 1 {
		writeErr(w, http.StatusForbidden, "bad_confirm", "confirmation code does not match")
		return
	}
	s.mu.Lock()
	at, ok := s.previews[previewKey(req.ReportID, req.RuleIDs)]
	s.mu.Unlock()
	if !ok || time.Since(at) > previewTTL {
		writeErr(w, http.StatusPreconditionRequired, "preview_required", "preview these changes first")
		return
	}
	todo, _, _, err := s.pickFixes(req.ReportID, req.RuleIDs)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such report")
		return
	}
	if len(todo) == 0 {
		writeErr(w, http.StatusBadRequest, "no_remediation", "none of the selected rules can be remediated")
		return
	}
	if s.jobs.busy() {
		writeErr(w, http.StatusConflict, "busy", "another scan or apply is already running")
		return
	}
	j := s.jobs.create("apply")
	j.update(func(v *JobView) { v.Phase = "snapshot"; v.Total = len(todo) })
	dead := time.Duration(s.deadman(req.DeadmanSec)) * time.Second
	s.store.log("fix.apply", fmt.Sprintf("%s rules=%d deadman=%s", req.ReportID, len(todo), dead), true)
	go s.runApply(j, todo, dead)
	writeJSON(w, http.StatusAccepted, j.view())
}

func (s *Server) runApply(j *Job, todo []rules.Rule, dead time.Duration) {
	mgr := snapshot.NewManager(s.opts.SnapshotDir)
	snapID, err := mgr.CreateSnapshot(todo)
	if err != nil {
		s.store.log("fix.failed", "snapshot: "+err.Error(), false)
		j.fail("snapshot_failed", err.Error())
		return
	}
	j.update(func(v *JobView) { v.SnapshotID = snapID; v.Phase = "applying" })

	runner := engine.NewRunnerWithConcurrency(1)
	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()
	out, err := capture(func() error { return runner.RunRemediation(ctx, todo, false) })
	j.update(func(v *JobView) { v.Output = out })
	if err != nil {
		rbOut, rbErr := capture(func() error { return mgr.Rollback(snapID) })
		msg := err.Error()
		if rbErr != nil {
			msg += " (rollback also failed: " + rbErr.Error() + ")"
		}
		s.store.log("fix.failed", msg, false)
		j.update(func(v *JobView) { v.Output += rbOut; v.RolledBack = rbErr == nil })
		j.fail("internal", msg)
		return
	}
	deadline := time.Now().UTC().Add(dead)
	j.update(func(v *JobView) { v.State = stateAwaiting; v.Phase = "waiting"; v.Deadline = &deadline })

	keep := false
	select {
	case keep = <-j.decision:
	case <-time.After(dead):
	}
	if keep {
		s.store.log("fix.confirmed", snapID, true)
		j.finish(stateDone)
		return
	}
	rbOut, rbErr := capture(func() error { return mgr.Rollback(snapID) })
	j.update(func(v *JobView) { v.Output += rbOut; v.RolledBack = rbErr == nil })
	if rbErr != nil {
		s.store.log("fix.rollback", rbErr.Error(), false)
		j.fail("restore_failed", rbErr.Error())
		return
	}
	s.store.log("fix.rolled_back", snapID, true)
	j.finish(stateRolled)
}

func (s *Server) decide(w http.ResponseWriter, r *http.Request, keep bool) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil || j.view().Kind != "apply" {
		writeErr(w, http.StatusNotFound, "not_found", "no such job")
		return
	}
	if j.view().State == stateAwaiting {
		select {
		case j.decision <- keep:
		default:
		}
		select {
		case <-j.finished:
		case <-time.After(20 * time.Second):
		}
	}
	writeJSON(w, http.StatusOK, j.view())
}

func (s *Server) handleJobConfirm(w http.ResponseWriter, r *http.Request)  { s.decide(w, r, true) }
func (s *Server) handleJobRollback(w http.ResponseWriter, r *http.Request) { s.decide(w, r, false) }

// handleRestore rolls the system back to a stored snapshot.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req fixReq
	if !decode(w, r, &req) {
		return
	}
	if !snapRe.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not_found", "no such snapshot")
		return
	}
	if !s.opts.IsElevated() {
		s.store.log("snapshot.restore", id+" refused: not elevated", false)
		writeErr(w, http.StatusForbidden, "not_elevated", "administrator/root privileges are required to restore")
		return
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(req.ConfirmCode))), []byte(s.nonce)) != 1 {
		writeErr(w, http.StatusForbidden, "bad_confirm", "confirmation code does not match")
		return
	}
	found := false
	for _, sn := range s.snapshots() {
		found = found || sn.ID == id
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "no such snapshot")
		return
	}
	if s.jobs.busy() {
		writeErr(w, http.StatusConflict, "busy", "another scan or apply is already running")
		return
	}
	out, err := capture(func() error { return snapshot.NewManager(s.opts.SnapshotDir).Rollback(id) })
	s.store.log("snapshot.restore", id, err == nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "restore_failed", "message": err.Error()}, "output": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": out})
}
