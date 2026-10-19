// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/snapshot"
	"github.com/tmaegis/tmaegis/internal/ui"
)

var (
	applyTarget     string
	applyLevel      int
	autoRollback    bool
	applyDryRun     bool
	deadmanSec      int
	noConfirm       bool
	applyAutoDetect bool
	terraformDir    string
)

var applyCmd = &cobra.Command{
	Use: "apply",
	RunE: func(cmd *cobra.Command, args []string) error {
		op := outUI()
		if op.Fancy() {
			op.Println(op.Banner(bannerInfo()))
		} else {
			fmt.Println(i18n.T("cli.apply.starting", "version", Version))
		}

		if !applyDryRun {
			fmt.Fprintln(os.Stderr, i18n.T("cli.apply.betaHint"))
		}

		loadedRules, host, err := selectRulesHost(applyTarget, applyAutoDetect, applyLevel)
		if err != nil {
			return withHint(lerr.Wrap("cli.scan.err.loadRules", err), rulesDirHint())
		}
		if osFamilyOverride != "" {
			host.Family = osFamilyOverride
		}
		if op.Fancy() {
			mode := i18n.T("cli.ui.mode.apply")
			if applyDryRun {
				mode = i18n.T("cli.ui.mode.dryRun")
			}
			op.Print(setupPanel(op, i18n.T("cli.ui.section.apply"), applyTarget, applyLevel, host, loadedRules,
				ui.KV{Key: i18n.T("cli.ui.kv.mode"), Value: op.Bold(mode)}))
		}

		runner := newRunner()
		runner.TerraformDir = terraformDir
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Only non-compliant controls are remediated.
		todo := auditWithProgress(func(progress func(rules.Rule)) *engine.ScanReport {
			runner.Progress = progress
			return runner.RunAudit(ctx, loadedRules)
		}, len(loadedRules)).FailedRules()
		if len(todo) == 0 {
			say(os.Stdout, i18n.T("cli.apply.nothingToDo"))
			return nil
		}
		say(os.Stdout, i18n.N("cli.apply.selected", len(todo)))
		if g := countGuidanceOnly(todo); g > 0 {
			say(os.Stdout, i18n.N("cli.apply.guidanceOnly", g))
		}

		writeChangeSet(todo)
		if applyDryRun {
			if err := runner.RunRemediation(ctx, todo, true); err != nil {
				return err
			}
			if op.Fancy() {
				cmdLine := "tmaegis apply"
				if applyTarget != "all" {
					cmdLine += " --target " + shellQuote(applyTarget)
				}
				if applyLevel != 1 {
					cmdLine += fmt.Sprintf(" --level %d", applyLevel)
				}
				op.Println("")
				op.Println(op.Section(i18n.T("cli.ui.next.title")))
				op.Println(op.Hint(i18n.T("cli.ui.next.applyForReal", "command", code(cmdLine))))
			}
			return nil
		}

		snapMgr := snapshot.NewManager(cfg.Engine.SnapshotDir)
		snapID, err := snapMgr.CreateSnapshot(todo)
		if err != nil {
			return lerr.Wrap("cli.apply.err.snapshot", err)
		}
		say(os.Stdout, i18n.T("cli.apply.snapshotCreated", "id", snapID))

		if err := runner.RunRemediation(ctx, todo, false); err != nil {
			say(os.Stdout, i18n.T("cli.apply.remediationError", "error", err))
			// Individual controls that failed are reported by the re-scan; only an aborted run rolls back.
		}
		if ctx.Err() != nil {
			err := ctx.Err()
			if autoRollback {
				say(os.Stdout, i18n.T("cli.apply.rollingBack", "id", snapID))
				if rbErr := snapMgr.Rollback(snapID); rbErr != nil {
					return lerr.Wrap("cli.apply.err.rollbackAlsoFailed", err, "rollbackError", rbErr)
				}
			}
			return err
		}

		say(os.Stdout, i18n.T("cli.apply.done"))
		verifyAfterApply(ctx, runner, todo)
		if noConfirm || deadmanSec <= 0 {
			return nil
		}
		return deadmanSwitch(snapMgr, snapID, time.Duration(deadmanSec)*time.Second)
	},
}

// countGuidanceOnly counts controls whose remediation is prose or IaC advice, which apply cannot execute.
func countGuidanceOnly(list []rules.Rule) int {
	n := 0
	for _, r := range list {
		if r.Remediation.Type == "iac_patch" || r.Remediation.Type == "" {
			n++
		}
	}
	return n
}

// writeChangeSet saves the guidance-only controls (network devices, cloud, SaaS, MDM, ...) as a
// Markdown change set, since a local CLI cannot change them directly.
func writeChangeSet(todo []rules.Rule) {
	var b strings.Builder
	n := 0
	for _, r := range todo {
		if r.Remediation.Type != "iac_patch" && r.Remediation.Type != "" {
			continue
		}
		n++
		fmt.Fprintf(&b, "## %s - %s\n\n%s\n\n", r.ID, r.Title, r.Remediation.Command)
	}
	if n == 0 {
		return
	}
	path := fmt.Sprintf("tmaegis-changeset-%s.md", time.Now().Format("20060102-150405"))
	if err := os.WriteFile(path, []byte("# TMAegis change set\n\nControls that need a change outside this host.\n\n"+b.String()), 0o644); err != nil {
		return
	}
	say(os.Stdout, i18n.T("cli.apply.changeset", "n", n, "path", path))
}

// verifyAfterApply re-audits the remediated controls so the operator sees what really changed.
func verifyAfterApply(ctx context.Context, runner *engine.Runner, todo []rules.Rule) {
	say(os.Stdout, i18n.T("cli.apply.verify.title"))
	runner.Progress = nil
	rep := runner.RunAudit(ctx, todo)
	fixed := 0
	var manual []string
	type fail struct{ id, why string }
	var auto []fail
	for _, res := range rep.Results {
		switch {
		case res.Status == engine.StatusPass:
			fixed++
		case res.Status == engine.StatusFail || res.Status == engine.StatusErr:
			if res.Rule.Remediation.Type == "iac_patch" || res.Rule.Remediation.Type == "" {
				manual = append(manual, res.Rule.ID)
			} else {
				auto = append(auto, fail{res.Rule.ID, res.Details})
			}
		}
	}
	say(os.Stdout, i18n.T("cli.apply.verify.result", "fixed", fixed, "total", len(todo)))
	if len(manual)+len(auto) == 0 {
		return
	}
	say(os.Stdout, i18n.T("cli.apply.verify.stillFailing", "n", len(manual)+len(auto), "manual", len(manual), "auto", len(auto)))
	shown := 0
	for _, a := range auto {
		if shown++; shown > 15 {
			break
		}
		fmt.Println(i18n.T("cli.apply.verify.item", "id", a.id, "reason", a.why))
	}
	for _, m := range manual {
		if shown++; shown > 15 {
			break
		}
		fmt.Println(i18n.T("cli.apply.verify.item", "id", m, "reason", i18n.T("cli.apply.verify.manual")))
	}
	if extra := len(manual) + len(auto) - 15; extra > 0 {
		fmt.Println(i18n.T("cli.apply.verify.more", "n", extra))
	}
}

// deadmanSwitch rolls back unless the operator confirms within the timeout. If a
// change locked the operator out (e.g. SSH), they cannot confirm and the host heals itself.
func deadmanSwitch(m *snapshot.Manager, snapID string, timeout time.Duration) error {
	word := i18n.T("cli.apply.confirmWord")
	say(os.Stdout, i18n.T("cli.apply.deadman.prompt", "word", word, "timeout", timeout))
	answer := make(chan bool, 1)
	go func() {
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer <- isConfirmWord(line, word)
	}()
	select {
	case ok := <-answer:
		if ok {
			say(os.Stdout, i18n.T("cli.apply.deadman.confirmed"))
			return nil
		}
	case <-time.After(timeout):
		say(os.Stdout, i18n.T("cli.apply.deadman.timeout"))
	}
	say(os.Stdout, i18n.T("cli.apply.deadman.rollingBack", "id", snapID))
	if err := m.Rollback(snapID); err != nil {
		return lerr.Wrap("cli.apply.err.deadmanRollback", err)
	}
	return lerr.New("cli.apply.err.notConfirmed")
}

// isConfirmWord accepts the English "confirm" in every language, plus the localized word.
func isConfirmWord(line, localized string) bool {
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "confirm") || strings.EqualFold(line, strings.TrimSpace(localized))
}

func init() {
	applyCmd.Flags().StringVar(&terraformDir, "terraform-out", "", "")
	applyCmd.Flags().BoolVar(&applyAutoDetect, "auto-detect", false, "")
	applyCmd.Flags().StringVarP(&applyTarget, "target", "t", "all", "")
	applyCmd.Flags().IntVarP(&applyLevel, "level", "l", 1, "")
	applyCmd.Flags().BoolVar(&autoRollback, "auto-rollback", true, "")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "")
	applyCmd.Flags().IntVar(&deadmanSec, "deadman-timeout", 60, "")
	applyCmd.Flags().BoolVar(&noConfirm, "no-confirm", false, "")
	rootCmd.AddCommand(applyCmd)
}
