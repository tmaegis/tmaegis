// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Job states.
const (
	stateRunning   = "running"
	stateAwaiting  = "awaiting_confirm" // apply finished; waiting for the dead-man confirmation
	stateDone      = "done"
	stateFailed    = "failed"
	stateCancelled = "cancelled"
	stateRolled    = "rolled_back"
)

// JobView is the JSON shape of a job, also streamed over Server-Sent Events.
type JobView struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"` // scan, apply
	State      string     `json:"state"`
	Phase      string     `json:"phase,omitempty"` // loading, auditing, snapshot, applying, waiting
	Total      int        `json:"total"`
	Done       int        `json:"done"`
	Passed     int        `json:"passed"`
	Failed     int        `json:"failed"`
	Current    string     `json:"current,omitempty"`
	Started    time.Time  `json:"started"`
	Ended      *time.Time `json:"ended,omitempty"`
	Error      string     `json:"error,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
	ReportID   string     `json:"report_id,omitempty"`
	SnapshotID string     `json:"snapshot_id,omitempty"`
	Deadline   *time.Time `json:"deadline,omitempty"`
	Output     string     `json:"output,omitempty"`
	RolledBack bool       `json:"rolled_back,omitempty"`
	// Log is the tail of the per-rule stream of a scan (newest last); Seq numbers every
	// finished rule so the UI can append only what it has not shown yet.
	Log     []StreamItem `json:"log,omitempty"`
	Skipped int          `json:"skipped"`
}

// StreamItem is one finished rule in the live scan stream.
type StreamItem struct {
	Seq      int    `json:"seq"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Target   string `json:"target"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
}

const streamTail = 40

func (v *JobView) push(it StreamItem) {
	v.Log = append(v.Log, it)
	if len(v.Log) > streamTail {
		v.Log = append([]StreamItem(nil), v.Log[len(v.Log)-streamTail:]...)
	}
}

// Job is a background task (scan or apply).
type Job struct {
	mu       sync.Mutex
	v        JobView
	notify   chan struct{}
	cancel   context.CancelFunc
	decision chan bool // apply: true keeps the changes, false rolls back
	finished chan struct{}
}

type jobs struct {
	mu    sync.Mutex
	m     map[string]*Job
	order []string
	seq   int
}

func newJobs() *jobs { return &jobs{m: map[string]*Job{}} }

func (js *jobs) create(kind string) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.seq++
	id := fmt.Sprintf("job-%d-%d", time.Now().Unix(), js.seq)
	j := &Job{
		v:      JobView{ID: id, Kind: kind, State: stateRunning, Started: time.Now().UTC()},
		notify: make(chan struct{}), decision: make(chan bool, 1), finished: make(chan struct{}),
	}
	js.m[id] = j
	js.order = append(js.order, id)
	return j
}

func (js *jobs) get(id string) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.m[id]
}

func (js *jobs) list() []JobView {
	js.mu.Lock()
	ids := append([]string(nil), js.order...)
	js.mu.Unlock()
	out := []JobView{}
	for _, id := range ids {
		if j := js.get(id); j != nil {
			v := j.view()
			v.Output = ""
			v.Log = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Started.After(out[b].Started) })
	return out
}

// busy reports whether a scan or apply is in flight; only one runs at a time.
func (js *jobs) busy() bool {
	js.mu.Lock()
	defer js.mu.Unlock()
	for _, j := range js.m {
		if st := j.view().State; st == stateRunning || st == stateAwaiting {
			return true
		}
	}
	return false
}

// rollbackPending rolls back every apply that is waiting for confirmation.
func (js *jobs) rollbackPending(_ string) {
	js.mu.Lock()
	var pending []*Job
	for _, j := range js.m {
		if j.view().State == stateAwaiting {
			pending = append(pending, j)
		}
	}
	js.mu.Unlock()
	for _, j := range pending {
		select {
		case j.decision <- false:
		default:
		}
		select {
		case <-j.finished:
		case <-time.After(15 * time.Second):
		}
	}
}

func (j *Job) view() JobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := j.v
	v.Log = append([]StreamItem(nil), j.v.Log...) // the caller may outlive the next update
	return v
}

// update mutates the job and wakes every event stream.
func (j *Job) update(fn func(v *JobView)) {
	j.mu.Lock()
	fn(&j.v)
	old := j.notify
	j.notify = make(chan struct{})
	j.mu.Unlock()
	close(old)
}

func (j *Job) wait() <-chan struct{} {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.notify
}

func (j *Job) finish(state string) {
	now := time.Now().UTC()
	j.update(func(v *JobView) { v.State = state; v.Ended = &now; v.Phase = ""; v.Current = ""; v.Deadline = nil })
	close(j.finished)
}

func (j *Job) fail(code, msg string) {
	j.update(func(v *JobView) { v.Error = msg; v.ErrorCode = code })
	j.finish(stateFailed)
}

// ---- HTTP ----

func (s *Server) handleJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.jobs.list()})
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such job")
		return
	}
	writeJSON(w, http.StatusOK, j.view())
}

func (s *Server) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such job")
		return
	}
	if j.view().Kind == "scan" && j.cancel != nil {
		j.cancel()
	}
	writeJSON(w, http.StatusOK, j.view())
}

// handleJobEvents streams the job as Server-Sent Events until it ends.
func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.get(r.PathValue("id"))
	if j == nil {
		writeErr(w, http.StatusNotFound, "not_found", "no such job")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		ch := j.wait()
		v := j.view()
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "event: job\ndata: %s\n\n", b)
		fl.Flush()
		s.touch()
		if v.State != stateRunning && v.State != stateAwaiting {
			return
		}
		select {
		case <-ch:
			select { // coalesce bursts of progress updates
			case <-time.After(80 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			fl.Flush()
			s.touch()
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		}
	}
}
