// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"net/http"

	"github.com/tmaegis/tmaegis/internal/snapshot"
)

type snapInfo struct {
	ID      string `json:"id"`
	Created string `json:"created"`
	Files   int    `json:"files"`
}

func (s *Server) snapshots() []snapInfo {
	list, _ := snapshot.NewManager(s.opts.SnapshotDir).List()
	out := make([]snapInfo, 0, len(list))
	for _, i := range list {
		out = append(out, snapInfo{ID: i.ID, Created: i.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), Files: i.Files})
	}
	return out
}

func (s *Server) handleSnapshots(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": s.snapshots()})
}

// handleHistory returns everything the History screen shows: reports, the score trend
// (oldest first, newest 30), snapshots and the audit trail of GUI actions.
func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	reps := s.store.reports()
	type pt struct {
		ID    string  `json:"id"`
		Time  string  `json:"time"`
		Score float64 `json:"score"`
	}
	trend := []pt{}
	for i := len(reps) - 1; i >= 0; i-- {
		trend = append(trend, pt{reps[i].ID, reps[i].Created.UTC().Format("2006-01-02T15:04:05Z"), reps[i].Score})
	}
	if len(trend) > 30 {
		trend = trend[len(trend)-30:]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": reps, "trend": trend, "snapshots": s.snapshots(), "actions": s.store.actions(100),
	})
}
