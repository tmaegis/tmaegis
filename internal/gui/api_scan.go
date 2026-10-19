// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

const scanTimeout = 20 * time.Minute

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var p ScanParams
	if !decode(w, r, &p) {
		return
	}
	if p.Scope == "" {
		p.Scope = "auto"
	}
	if p.Scope != "auto" && p.Scope != "target" && p.Scope != "dir" {
		writeErr(w, http.StatusBadRequest, "bad_request", "unknown scope")
		return
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
	if p.DeviceConfig != "" && s.store.deviceConfigPath(p.DeviceConfig) == "" {
		writeErr(w, http.StatusBadRequest, "no_device_config", "the device configuration was not found; upload it again")
		return
	}
	j, ok := s.startScan(p)
	if !ok {
		writeErr(w, http.StatusConflict, "busy", "another scan or apply is already running")
		return
	}
	writeJSON(w, http.StatusAccepted, j.view())
}

func (s *Server) rulesDir() string {
	if d := s.store.settings().RulesDir; d != "" {
		return d
	}
	return s.opts.RulesDir
}

// startScan creates the job and runs it in the background; ok is false when busy.
func (s *Server) startScan(p ScanParams) (*Job, bool) {
	if s.jobs.busy() {
		return nil, false
	}
	j := s.jobs.create("scan")
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	j.cancel = cancel
	j.update(func(v *JobView) { v.Phase = "loading" })
	s.store.log("scan.start", describe(p), true)
	go func() {
		defer cancel()
		s.runScan(ctx, j, p)
	}()
	return j, true
}

func describe(p ScanParams) string {
	d := fmt.Sprintf("scope=%s target=%s level=%d dir=%s manual=%t", p.Scope, p.Target, p.Level, p.RulesDir, p.IncludeManual)
	if p.DeviceConfig != "" {
		d += " device_config=" + p.DeviceName
	}
	return d
}

func (s *Server) runScan(ctx context.Context, j *Job, p ScanParams) {
	host := s.detectHost(false)
	var list []rules.Rule
	all, err := s.loadAll(p.RulesDir, true)
	if err == nil {
		list, err = selectFrom(all, p, host)
	}
	if err != nil {
		s.store.log("scan.failed", err.Error(), false)
		j.fail("rules_load_failed", err.Error())
		return
	}
	if len(list) == 0 {
		s.store.log("scan.failed", "no matching rules", false)
		j.fail("no_rules", "no rules match this scope and level")
		return
	}
	j.update(func(v *JobView) { v.Phase = "auditing"; v.Total = len(list) })

	runner := engine.NewRunnerWithConcurrency(s.opts.Concurrency)
	if p.DeviceConfig != "" {
		runner.DeviceConfig = s.store.deviceConfigPath(p.DeviceConfig)
	}
	results := make([]engine.RuleResult, len(list))
	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < s.opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				if ctx.Err() != nil {
					continue
				}
				r := list[i]
				j.update(func(v *JobView) { v.Current = r.ID + "  " + r.Title })
				// One rule per RunAudit call gives per-rule progress without changing the engine.
				res := runner.RunAudit(ctx, []rules.Rule{r})
				results[i] = res.Results[0]
				j.update(func(v *JobView) {
					v.Done++
					switch results[i].Status {
					case engine.StatusPass:
						v.Passed++
					case engine.StatusFail, engine.StatusErr:
						v.Failed++
					case engine.StatusSkip:
						v.Skipped++
					}
					v.push(StreamItem{Seq: v.Done, ID: r.ID, Title: r.Title, Target: r.Target, Severity: r.Severity, Status: string(results[i].Status)})
				})
			}
		}()
	}
feed:
	for i := range list {
		select {
		case idx <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(idx)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		s.store.log("scan.cancelled", "", true)
		if err == context.DeadlineExceeded {
			j.fail("internal", "scan timed out")
			return
		}
		j.finish(stateCancelled)
		return
	}

	rep := &engine.ScanReport{Timestamp: time.Now().UTC(), OS: runtime.GOOS, Arch: runtime.GOARCH, TotalRules: len(list), Results: results}
	rep.Hostname, _ = os.Hostname()
	for _, res := range results {
		switch res.Status {
		case engine.StatusPass:
			rep.PassedCount++
		case engine.StatusFail, engine.StatusErr:
			rep.FailedCount++
		}
	}
	sr := &StoredReport{ID: newReportID(), Created: rep.Timestamp, Params: p, Score: rep.Score(), Report: rep}
	if err := s.store.saveReport(sr); err != nil {
		s.store.log("scan.failed", err.Error(), false)
		j.fail("internal", err.Error())
		return
	}
	s.store.log("scan.done", fmt.Sprintf("%s score=%.1f passed=%d failed=%d", sr.ID, sr.Score, rep.PassedCount, rep.FailedCount), true)
	j.update(func(v *JobView) { v.ReportID = sr.ID })
	j.finish(stateDone)
}
