// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"context"
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/ai"
	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/plugin"
	"github.com/tmaegis/tmaegis/internal/preflight"
	reportpkg "github.com/tmaegis/tmaegis/internal/report"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/ui"
)

var (
	scanTarget        string
	scanLevel         int
	enableAI          bool
	triggerPlugin     string
	scanFormat        string
	scanAutoDetect    bool
	pushURL           string
	scanOutput        string
	scanDeviceConfig  string
	scanIncludeManual bool
	scanQuiet         bool
)

var scanCmd = &cobra.Command{
	Use: "scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		if remoteRequested(cmd) {
			return runRemoteScan(cmd)
		}
		// Progress goes to stderr so json/sarif on stdout stays machine-readable.
		ep := errUI()
		if !scanQuiet && scanFormat == "text" {
			PrintBanner(os.Stderr)
		}
		if !ep.Fancy() {
			fmt.Fprintln(os.Stderr, i18n.T("cli.scan.starting", "version", Version))
			fmt.Fprintln(os.Stderr, i18n.T("cli.scan.filter", "target", scanTarget, "level", scanLevel, "dir", rulesDir))
			fmt.Fprintln(os.Stderr)
		}

		loadedRules, host, err := selectRulesHost(scanTarget, scanAutoDetect, scanLevel)
		if err != nil {
			return withHint(lerr.Wrap("cli.scan.err.loadRules", err), rulesDirHint())
		}
		if osFamilyOverride != "" {
			host.Family = osFamilyOverride
		}
		loadedRules = scopeRules(loadedRules, scanDeviceConfig != "", scanIncludeManual)
		if len(loadedRules) == 0 {
			say(os.Stderr, i18n.T("cli.scan.noRules"))
			if ep.Fancy() {
				ep.Println(ep.Hint(i18n.T("cli.ui.hint.noRules", "command", code("tmaegis rules list"))))
			}
			return nil
		}
		if ep.Fancy() {
			var extra []ui.KV
			if scanDeviceConfig != "" {
				extra = append(extra, ui.KV{Key: i18n.T("cli.ui.kv.deviceConfig"), Value: scanDeviceConfig})
			}
			if scanFormat != "text" {
				extra = append(extra, ui.KV{Key: i18n.T("cli.ui.kv.format"), Value: scanFormat})
			}
			ep.Print(setupPanel(ep, i18n.T("cli.ui.section.scan"), scanTarget, scanLevel, host, loadedRules, extra...))
		}

		runner := newRunner()
		runner.DeviceConfig = scanDeviceConfig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		started := time.Now()
		report := auditWithProgress(func(progress func(rules.Rule)) *engine.ScanReport {
			runner.Progress = progress
			return runner.RunAudit(ctx, loadedRules)
		}, len(loadedRules))
		var extra []string
		if scanDeviceConfig != "" {
			extra = append(extra, "--device-config")
		}
		opts := summaryOptions{duration: time.Since(started), device: scanDeviceConfig != "", format: scanFormat, flags: scanFlags(cmd)}
		if scanFormat != "text" {
			summaryFollows = true
			err := writeReport(report, scanFormat, scanOutput, started, extra)
			summaryFollows = false
			if err != nil {
				return err
			}
			if scanOutput != "" {
				opts.reports = []string{scanOutput}
			}
		}
		finishScan(report, opts, os.Stdout)
		postScan(ctx, report, preflight.Collect)
		return nil
	},
}

// scanFlags repeats the options that select the scanned rules, for suggested commands.
func scanFlags(cmd *cobra.Command) string {
	var parts []string
	if scanTarget != "all" {
		parts = append(parts, "--target "+scanTarget)
	}
	if scanAutoDetect {
		parts = append(parts, "--auto-detect")
	}
	if scanLevel != 1 {
		parts = append(parts, fmt.Sprintf("--level %d", scanLevel))
	}
	if osFamilyOverride != "" {
		parts = append(parts, "--os-family "+shellQuote(osFamilyOverride))
	}
	if f := cmd.Flags().Lookup("rules-dir"); f != nil && f.Changed {
		parts = append(parts, "-r "+shellQuote(rulesDir))
	}
	if scanDeviceConfig != "" {
		parts = append(parts, "--device-config "+shellQuote(scanDeviceConfig))
	}
	return strings.Join(parts, " ")
}

// shellQuote quotes s for a POSIX shell when it contains anything unusual.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./:=@") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// reportOptions lists the scan options recorded in a report.
func reportOptions(extra []string) []string {
	opts := []string{fmt.Sprintf("--level %d", scanLevel)}
	if scanTarget != "all" {
		opts = append(opts, "--target "+scanTarget)
	}
	if scanAutoDetect {
		opts = append(opts, "--auto-detect")
	}
	if osFamilyOverride != "" {
		opts = append(opts, "--os-family "+osFamilyOverride)
	}
	if scanIncludeManual {
		opts = append(opts, "--include-manual")
	}
	return append(opts, extra...)
}

// writeReport writes report in a machine format to output (stdout when empty).
func writeReport(report *engine.ScanReport, format, output string, started time.Time, extra []string) error {
	if (format == "xlsx" || format == "pdf") && output == "" {
		return withHint(lerr.New("cli.scan.err.outputRequired", "format", format),
			i18n.T("cli.ui.hint.output", "command", code("-o tmaegis-report."+format)))
	}
	if id := os.Getenv("AWS_ACCOUNT_ID"); id != "" {
		reportpkg.ASFFOptions.AccountID = id
	}
	if region := os.Getenv("AWS_REGION"); region != "" {
		reportpkg.ASFFOptions.Region = region
	}
	out := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	info := reportpkg.Info{Version: Version, RulesDir: rulesDir, Duration: time.Since(started), Options: reportOptions(extra)}
	if err := reportpkg.WriteWith(out, format, report, i18n.Lang(), info); err != nil {
		return err
	}
	if output != "" && !(summaryFollows && errUI().Fancy()) {
		say(os.Stderr, i18n.N("cli.scan.reportWritten", report.TotalRules, "format", format, "output", output,
			"passed", report.PassedCount, "failed", report.FailedCount, "score", fmt.Sprintf("%.1f", report.Score())))
	}
	return nil
}

// postScan runs the optional AI analysis, report push and plugins. hostContext is the
// pre-flight snapshot given to the AI (nil for a remote host: the local one would mislead it).
func postScan(ctx context.Context, report *engine.ScanReport, hostContext func(context.Context) string) {
	msgOut := os.Stdout
	if scanFormat != "text" {
		msgOut = os.Stderr // keep json/sarif on stdout machine-readable
	}
	if enableAI {
		fmt.Fprintln(msgOut)
		say(msgOut, i18n.T("cli.scan.ai.running"))
		providerName := aiProvider
		if providerName == "" {
			providerName = os.Getenv("TMAEGIS_AI_PROVIDER")
		}
		if providerName == "" {
			providerName = cfg.AI.Provider
		}
		model := firstNonEmpty(os.Getenv("TMAEGIS_AI_MODEL"), cfg.AI.Model)
		endpoint := firstNonEmpty(os.Getenv("TMAEGIS_AI_ENDPOINT"), cfg.AI.Endpoint)
		apiKey := os.Getenv("TMAEGIS_AI_API_KEY")
		if cfg.AI.APIKeyEnv != "" && apiKey == "" {
			apiKey = os.Getenv(cfg.AI.APIKeyEnv)
		}

		hostCtx := ""
		if hostContext != nil {
			hostCtx = hostContext(ctx)
		}
		analyzer, err := ai.NewAnalyzer(ai.Config{
			Provider:    providerName,
			APIKey:      apiKey,
			Model:       model,
			Endpoint:    endpoint,
			HostContext: hostCtx,
		})
		if err != nil {
			say(msgOut, i18n.T("cli.scan.ai.initWarning", "error", err))
		} else {
			analysis, err := analyzer.AnalyzeScanReport(ctx, report)
			if err != nil {
				say(msgOut, i18n.T("cli.scan.ai.failed", "error", err))
			} else {
				fmt.Fprintf(msgOut, "\n%s\n%s\n", i18n.T("cli.scan.ai.heading", "provider", analyzer.Name()), analysis)
			}
		}
	}

	if pushURL != "" {
		if err := pushReport(ctx, pushURL, os.Getenv("TMAEGIS_SERVER_TOKEN"), report); err != nil {
			say(os.Stderr, i18n.T("cli.scan.pushFailed", "error", err))
		} else {
			say(os.Stderr, i18n.T("cli.scan.pushed", "url", pushURL))
		}
	}

	names := cfg.Plugins.Enabled
	if triggerPlugin != "" {
		names = strings.Split(triggerPlugin, ",")
	}
	pm := plugin.NewManager(cfg.Plugins.Directory)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		fmt.Fprintln(os.Stderr)
		say(os.Stderr, i18n.T("cli.scan.plugin.running", "name", name))
		if err := pm.Execute(ctx, name, report); err != nil {
			say(os.Stderr, i18n.T("cli.scan.plugin.error", "error", err))
		}
	}
}

// scopeRules keeps the report about the host being scanned: device rules (config_regex)
// only run against a supplied --device-config, and when one is supplied only device rules
// run; manual-review rules are listed only on request.
func scopeRules(in []rules.Rule, device, manual bool) []rules.Rule {
	var out []rules.Rule
	for _, r := range in {
		if (r.Audit.Type == "config_regex") != device {
			continue
		}
		if r.Audit.Type == "manual" && !manual {
			continue
		}
		out = append(out, r)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	flagArgs["scan.format"] = []any{"formats", strings.Join(reportpkg.Formats, ", ")}
	scanCmd.Flags().StringVar(&pushURL, "push-url", "", "")
	scanCmd.Flags().BoolVar(&scanAutoDetect, "auto-detect", false, "")
	scanCmd.Flags().StringVarP(&scanFormat, "format", "f", "text", "")
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "", "")
	scanCmd.Flags().StringVarP(&scanTarget, "target", "t", "all", "")
	scanCmd.Flags().IntVarP(&scanLevel, "level", "l", 1, "")
	scanCmd.Flags().BoolVar(&enableAI, "ai-analyze", false, "")
	scanCmd.Flags().StringVar(&triggerPlugin, "plugin", "", "")
	scanCmd.Flags().StringVar(&scanDeviceConfig, "device-config", "", "")
	scanCmd.Flags().BoolVar(&scanIncludeManual, "include-manual", false, "")
	scanCmd.Flags().BoolVarP(&scanQuiet, "quiet", "q", false, "")
	rootCmd.AddCommand(scanCmd)
}
