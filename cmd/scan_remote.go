// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"github.com/tmaegis/tmaegis/internal/remote"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// Remote (SSH) scan options. Phase 1 is strictly read-only: nothing here can change a host.
var (
	sshDest       string
	sshKey        string
	sshKnownHosts string
	sshTimeout    time.Duration
	sshJump       string
	sshInsecure   bool
	sshCommands   []string
	sshInventory  string
	sshParallel   int
)

// sshFlags are the options that only make sense with --ssh or --ssh-inventory.
var sshFlags = []string{"ssh-key", "ssh-known-hosts", "ssh-timeout", "ssh-jump", "ssh-insecure-ignore-host-key", "ssh-command", "ssh-parallel"}

// remoteDial is replaced in tests (prompt and environment isolation).
var remoteDial = remote.Dial

// remoteConfig builds the connection settings; tests swap it to avoid the real terminal and agent.
var remoteConfig = func() remote.Config {
	return remote.Config{KeyFile: sshKey, KnownHosts: sshKnownHosts, InsecureIgnoreHostKey: sshInsecure, ConnectTimeout: sshTimeout}
}

func remoteRequested(cmd *cobra.Command) bool {
	if sshDest != "" || sshInventory != "" {
		return true
	}
	for _, f := range sshFlags {
		if cmd.Flags().Changed(f) {
			return true // reported as an error by runRemoteScan
		}
	}
	return false
}

func localUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		name := u.Username
		if i := strings.LastIndex(name, `\`); i >= 0 {
			name = name[i+1:] // DOMAIN\user on Windows
		}
		return name
	}
	return firstNonEmpty(os.Getenv("USER"), os.Getenv("USERNAME"))
}

// remoteJob is one host to scan.
type remoteJob struct {
	ep     remote.Endpoint
	target string
}

type remoteResult struct {
	job    remoteJob
	report *engine.ScanReport
	text   bytes.Buffer // progress and text-format output, printed in inventory order
	err    error
	// duration is shown in the summary card.
	duration time.Duration
}

func runRemoteScan(cmd *cobra.Command) error {
	if sshDest == "" && sshInventory == "" {
		return lerr.New("cli.scan.ssh.err.needDestination")
	}
	if sshDest != "" && sshInventory != "" {
		return lerr.New("cli.scan.ssh.err.conflict")
	}
	if scanDeviceConfig != "" {
		return lerr.New("cli.scan.ssh.err.deviceConfig")
	}
	if sshParallel < 1 || sshParallel > 16 {
		return lerr.New("cli.scan.ssh.err.parallel")
	}
	if !scanQuiet && scanFormat == "text" {
		PrintBanner(os.Stderr)
	}
	if !scanQuiet {
		say(os.Stderr, i18n.T("cli.scan.ssh.authReminder"))
	}
	say(os.Stderr, i18n.T("cli.scan.starting", "version", Version))
	if sshInsecure {
		// Always shown, even with --quiet.
		say(os.Stderr, i18n.T("cli.scan.ssh.insecureWarning"))
	}
	if os.Getenv(remote.PasswordEnv) != "" {
		say(os.Stderr, i18n.T("cli.scan.ssh.passwordEnv", "env", remote.PasswordEnv))
	}

	var jobs []remoteJob
	if sshDest != "" {
		ep, err := remote.ParseEndpoint(sshDest, localUser())
		if err != nil {
			return lerr.Wrap("cli.scan.ssh.err.destination", err, "dest", sshDest)
		}
		jobs = append(jobs, remoteJob{ep: ep, target: strings.ToLower(scanTarget)})
	} else {
		inv, err := remote.LoadInventory(sshInventory, localUser())
		if err != nil {
			return lerr.Wrap("cli.scan.ssh.err.inventory", err, "path", sshInventory)
		}
		for _, e := range inv {
			jobs = append(jobs, remoteJob{ep: e.Endpoint, target: firstNonEmpty(e.Target, strings.ToLower(scanTarget))})
		}
		if scanFormat != "text" || scanOutput != "" {
			if scanOutput == "" {
				return lerr.New("cli.scan.ssh.err.outputDir", "format", scanFormat)
			}
			if err := os.MkdirAll(scanOutput, 0o755); err != nil {
				return err
			}
		}
	}
	// Device commands are checked before any connection is made.
	for _, j := range jobs {
		if remote.IsDevice(j.target) {
			if _, err := remote.PlanFor(j.target, sshCommands); err != nil {
				return err
			}
		} else if len(sshCommands) > 0 {
			return lerr.New("cli.scan.ssh.err.commandForHost", "target", j.target)
		}
	}
	fmt.Fprintln(os.Stderr)

	results := make([]*remoteResult, len(jobs))
	sem := make(chan struct{}, sshParallel)
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, j remoteJob) {
			defer wg.Done()
			defer func() { <-sem }()
			res := &remoteResult{job: j}
			var log io.Writer = os.Stderr
			if len(jobs) > 1 {
				log = &res.text // keep each host's lines together
			}
			started := time.Now()
			res.report, res.err = scanRemoteHost(j, log)
			res.duration = time.Since(started)
			results[i] = res
		}(i, j)
	}
	wg.Wait()

	failed := 0
	for _, res := range results {
		if len(jobs) > 1 {
			hostHeader(os.Stderr, i18n.T("cli.scan.ssh.hostHeader", "host", res.job.ep.String(), "target", res.job.target))
			for _, l := range strings.SplitAfter(res.text.String(), "\n") {
				if l != "" {
					say(os.Stderr, strings.TrimSuffix(l, "\n"))
				}
			}
		}
		if res.err != nil {
			failed++
			if len(jobs) == 1 {
				return lerr.Wrap("cli.scan.ssh.err.host", res.err, "host", res.job.ep.String())
			}
			say(os.Stderr, i18n.T("cli.scan.ssh.hostError", "host", res.job.ep.String(), "error", res.err))
			continue
		}
		if err := emitRemoteReport(res, len(jobs) > 1); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		postScan(ctx, res.report, nil)
		cancel()
	}
	if len(jobs) > 1 {
		printRemoteSummary(os.Stderr, results)
	}
	if failed > 0 {
		return lerr.New("cli.scan.ssh.err.hostsFailed", "failed", failed, "total", len(jobs))
	}
	return nil
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// reportPath names one host's report inside the --output directory.
func reportPath(dir string, ep remote.Endpoint, format string) string {
	name := unsafeFileChars.ReplaceAllString(ep.Host, "_")
	if ep.Port != 22 {
		name += "_" + strconv.Itoa(ep.Port)
	}
	ext := format
	if format == "text" {
		ext = "txt"
	}
	return filepath.Join(dir, name+"."+ext)
}

func emitRemoteReport(res *remoteResult, multi bool) error {
	extra := []string{"--ssh " + res.job.ep.String()}
	if res.job.target != strings.ToLower(scanTarget) {
		extra = append(extra, "--target "+res.job.target)
	}
	output := scanOutput
	if multi && output != "" {
		output = reportPath(scanOutput, res.job.ep, scanFormat)
	}
	if scanFormat == "text" {
		if output == "" {
			finishScan(res.report, summaryOptions{remote: true, format: "text", duration: res.duration}, os.Stdout)
			return nil
		}
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		defer f.Close()
		res.report.WriteSummary(f)
		say(os.Stderr, i18n.N("cli.scan.reportWritten", res.report.TotalRules, "format", scanFormat, "output", output,
			"passed", res.report.PassedCount, "failed", res.report.FailedCount, "score", fmt.Sprintf("%.1f", res.report.Score())))
		return nil
	}
	return writeReport(res.report, scanFormat, output, res.report.Timestamp, extra)
}

func printRemoteSummary(w io.Writer, results []*remoteResult) {
	if p := printerFor(w); p.Fancy() {
		printRemoteSummaryTable(p, results)
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.N("cli.scan.ssh.summary.title", len(results)))
	for _, res := range results {
		host := fmt.Sprintf("%-28s %-14s", res.job.ep.String(), res.job.target)
		if res.err != nil {
			fmt.Fprintln(w, "  "+i18n.T("cli.scan.ssh.summary.error", "host", host, "error", res.err))
			continue
		}
		r := res.report
		fmt.Fprintln(w, "  "+i18n.T("cli.scan.ssh.summary.row", "host", host,
			"passed", r.PassedCount, "failed", r.FailedCount, "total", r.TotalRules, "score", fmt.Sprintf("%.1f", r.Score())))
	}
}

// scanRemoteHost connects to one host, selects its rules and audits it read-only.
func scanRemoteHost(j remoteJob, log io.Writer) (*engine.ScanReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cfg := remoteConfig()
	if sshJump != "" {
		jump, err := remote.ParseEndpoint(sshJump, j.ep.User)
		if err != nil {
			return nil, lerr.Wrap("cli.scan.ssh.err.destination", err, "dest", sshJump)
		}
		cfg.Jump = &jump
	}
	say(log, i18n.T("cli.scan.ssh.connecting", "host", j.ep.String()))
	client, err := remoteDial(ctx, j.ep, cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	runner := newRunner()
	var ruleList []rules.Rule
	started := time.Now()
	if remote.IsDevice(j.target) {
		plan, err := remote.PlanFor(j.target, sshCommands)
		if err != nil {
			return nil, err
		}
		say(log, i18n.T("cli.scan.ssh.fetching", "vendor", plan.Vendor.Name, "commands", strings.Join(plan.Commands, "; ")))
		conf, err := client.FetchConfig(ctx, plan)
		if err != nil {
			return nil, err
		}
		say(log, i18n.T("cli.scan.ssh.fetched", "bytes", len(conf)))
		ruleList, err = rules.LoadFiltered(rulesDir, rules.Filter{Targets: []string{j.target}, MaxLevel: scanLevel, OSFamily: osFamilyOverride})
		if err != nil {
			return nil, lerr.Wrap("cli.scan.err.loadRules", err)
		}
		ruleList = scopeRules(ruleList, true, scanIncludeManual)
		runner.DeviceConfigData = conf
		runner.DeviceConfigSource = "ssh://" + j.ep.String()
		runner.Exec = remote.DeviceExecutor{Info: engine.Target{Hostname: j.ep.Host, OS: j.target}}
	} else {
		host, info := remote.DetectHost(ctx, client)
		if osFamilyOverride != "" {
			host.Family = osFamilyOverride
		}
		if info.Hostname == "" {
			info.Hostname = j.ep.Host
		}
		say(log, i18n.T("cli.scan.ssh.detected", "host", j.ep.String(), "os", firstNonEmpty(host.OS, "?"), "family", firstNonEmpty(host.Family, "?")))
		f := rules.Filter{MaxLevel: scanLevel, OSFamily: host.Family}
		switch {
		case j.target != "all":
			f.Targets = []string{j.target}
		case host.OS == "":
			return nil, lerr.New("cli.scan.ssh.err.noOS", "host", j.ep.String())
		case scanAutoDetect:
			f.Targets = host.Targets() // OS only: installed software is not probed remotely yet
		}
		ruleList, err = rules.LoadFiltered(rulesDir, f)
		if err != nil {
			return nil, lerr.Wrap("cli.scan.err.loadRules", err)
		}
		if j.target == "all" && !scanAutoDetect {
			ruleList = forThisHost(ruleList, host.Targets()[0])
		}
		ruleList = scopeRules(ruleList, false, scanIncludeManual)
		runner.Exec = &remote.HostExecutor{Client: client, Info: info}
	}
	if len(ruleList) == 0 {
		say(log, i18n.T("cli.scan.noRules"))
	}
	report := runner.RunAudit(ctx, ruleList)
	report.Timestamp = started.UTC()
	return report, nil
}

func init() {
	f := scanCmd.Flags()
	f.StringVar(&sshDest, "ssh", "", "")
	f.StringVar(&sshKey, "ssh-key", "", "")
	f.StringVar(&sshKnownHosts, "ssh-known-hosts", "", "")
	f.DurationVar(&sshTimeout, "ssh-timeout", remote.DefaultConnectTimeout, "")
	f.StringVar(&sshJump, "ssh-jump", "", "")
	f.BoolVar(&sshInsecure, "ssh-insecure-ignore-host-key", false, "")
	f.StringArrayVar(&sshCommands, "ssh-command", nil, "")
	f.StringVar(&sshInventory, "ssh-inventory", "", "")
	f.IntVar(&sshParallel, "ssh-parallel", 1, "")
}
