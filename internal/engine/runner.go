// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"github.com/tmaegis/tmaegis/internal/providers/cloud"
	"github.com/tmaegis/tmaegis/internal/rules"
)

type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
	StatusErr  Status = "ERROR"
)

type RuleResult struct {
	Rule    rules.Rule `json:"rule"`
	Status  Status     `json:"status"`
	Details string     `json:"details"`
}

type ScanReport struct {
	Timestamp   time.Time    `json:"timestamp"`
	Hostname    string       `json:"hostname,omitempty"`
	OS          string       `json:"os"`
	Arch        string       `json:"arch"`
	TotalRules  int          `json:"total_rules"`
	PassedCount int          `json:"passed_count"`
	FailedCount int          `json:"failed_count"`
	Results     []RuleResult `json:"results"`
}

// PrintSummary writes the human-readable report to stdout.
func (r *ScanReport) PrintSummary() { r.WriteSummary(os.Stdout) }

// WriteSummary writes the human-readable report to w. Status badges stay English so
// scripts can keep grepping them; the surrounding text follows the active language.
func (r *ScanReport) WriteSummary(w io.Writer) {
	const rule = "======================================================================"
	fmt.Fprintln(w, rule)
	fmt.Fprintln(w, i18n.T("cli.engine.summary.title", "os", r.OS, "arch", r.Arch, "time", r.Timestamp.Format(time.RFC3339)))
	fmt.Fprintln(w, rule)
	for _, res := range r.Results {
		badge := "[PASS]"
		if res.Status == StatusFail {
			badge = "[FAIL]"
		} else if res.Status == StatusSkip {
			badge = "[SKIP]"
		} else if res.Status == StatusErr {
			badge = "[ERR ]"
		}
		fmt.Fprintf(w, "%-6s %-24s %s\n", badge, res.Rule.ID, res.Rule.Title)
		if res.Status != StatusPass && res.Details != "" {
			fmt.Fprintln(w, i18n.T("cli.engine.summary.details", "details", res.Details))
		}
	}
	fmt.Fprintln(w, "----------------------------------------------------------------------")
	fmt.Fprintln(w, i18n.T("cli.engine.summary.totals",
		"total", r.TotalRules, "passed", r.PassedCount, "failed", r.FailedCount, "score", fmt.Sprintf("%.1f", r.Score())))
	fmt.Fprintln(w, rule)
}

// Score is the compliance percentage over evaluated rules. Skipped (not applicable)
// rules are excluded so a worker node is not penalised for missing control-plane files.
func (r *ScanReport) Score() float64 {
	evaluated := r.PassedCount + r.FailedCount
	if evaluated == 0 {
		return 0
	}
	return float64(r.PassedCount) / float64(evaluated) * 100
}

type Runner struct {
	concurrency int
	// TerraformDir, when set, receives .tf files proposed for cloud_check rules.
	TerraformDir string
	// DeviceConfig is an exported device configuration (e.g. Cisco "show running-config")
	// that config_regex rules are evaluated against. Without it those rules are skipped.
	DeviceConfig string
	// DeviceConfigData, when non-nil, is a configuration already captured in memory (e.g.
	// fetched over SSH); it takes precedence over DeviceConfig. DeviceConfigSource names it
	// in findings.
	DeviceConfigData   []byte
	DeviceConfigSource string
	// Exec runs commands and reads files; nil means this machine. A Remote executor also
	// supplies the report's host, OS and architecture, and blocks remediation.
	Exec Executor
	// Progress, when set, is called after each rule is audited with the rule just finished.
	// Audits run in parallel, so it must be safe for concurrent use.
	Progress func(r rules.Rule)
}

func (run *Runner) executor() Executor {
	if run.Exec == nil {
		return LocalExecutor{}
	}
	return run.Exec
}

// isRemote reports whether audits run on another machine.
func (run *Runner) isRemote() bool {
	_, ok := run.Exec.(Remote)
	return ok
}

// NewRunner returns a runner that audits rules sequentially.
func NewRunner() *Runner { return &Runner{concurrency: 1} }

// NewRunnerWithConcurrency audits up to n rules in parallel. Audits are read-only,
// so this is safe; remediation always runs sequentially and in rule order.
func NewRunnerWithConcurrency(n int) *Runner {
	if n < 1 {
		n = 1
	}
	return &Runner{concurrency: n}
}

func (run *Runner) RunAudit(ctx context.Context, ruleList []rules.Rule) *ScanReport {
	report := &ScanReport{
		Timestamp:  time.Now().UTC(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		TotalRules: len(ruleList),
	}
	report.Hostname, _ = os.Hostname()
	if rem, ok := run.Exec.(Remote); ok {
		t := rem.Target()
		report.Hostname, report.OS, report.Arch = t.Hostname, t.OS, t.Arch
	}

	results := make([]RuleResult, len(ruleList))
	sem := make(chan struct{}, run.concurrency)
	var wg sync.WaitGroup
	for i, rule := range ruleList {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, rule rules.Rule) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = run.evaluateRule(ctx, rule)
			if run.Progress != nil {
				run.Progress(rule)
			}
		}(i, rule)
	}
	wg.Wait()

	for _, res := range results {
		if res.Status == StatusPass {
			report.PassedCount++
		} else if res.Status == StatusFail || res.Status == StatusErr {
			report.FailedCount++
		}
	}
	report.Results = results

	return report
}

func (run *Runner) evaluateRule(ctx context.Context, r rules.Rule) RuleResult {
	ex := run.executor()
	if len(r.Audit.RequiresFile) > 0 && !anyExists(ctx, ex, r.Audit.RequiresFile) {
		return RuleResult{Rule: r, Status: StatusSkip, Details: "not applicable: " + strings.Join(r.Audit.RequiresFile, ", ") + " not present"}
	}
	switch r.Audit.Type {
	case "file_regex":
		content, err := ex.ReadFile(ctx, r.Audit.FilePath)
		if err != nil {
			return RuleResult{Rule: r, Status: StatusFail, Details: fmt.Sprintf("cannot read file %s: %v", r.Audit.FilePath, err)}
		}
		return matchContent(r, content, r.Audit.FilePath)

	case "config_regex":
		if run.DeviceConfigData != nil {
			return matchContent(r, run.DeviceConfigData, run.DeviceConfigSource)
		}
		if run.DeviceConfig == "" {
			return RuleResult{Rule: r, Status: StatusSkip, Details: "device rule: pass --device-config <exported configuration>"}
		}
		content, err := os.ReadFile(run.DeviceConfig)
		if err != nil {
			return RuleResult{Rule: r, Status: StatusErr, Details: fmt.Sprintf("cannot read device config %s: %v", run.DeviceConfig, err)}
		}
		return matchContent(r, content, run.DeviceConfig)

	case "manual":
		return RuleResult{Rule: r, Status: StatusSkip, Details: "manual review: " + r.Audit.Command}

	case "command", "sql_query", "cloud_cel", "registry":
		if r.Audit.Type == "sql_query" && r.Shell() != "powershell" {
			if tool := sqlClient(r.Audit.Command); tool != "" {
				if _, err := ex.Run(ctx, "command -v "+tool, "sh"); err != nil {
					return RuleResult{Rule: r, Status: StatusSkip, Details: "not applicable: " + tool + " not installed"}
				}
			}
		}
		out, err := ex.Run(ctx, r.Audit.Command, r.Shell())
		outputStr := strings.TrimSpace(string(out))
		if err != nil && strings.Contains(err.Error(), "exit status 127") {
			return RuleResult{Rule: r, Status: StatusSkip, Details: "not applicable: required tool not installed (" + outputStr + ")"}
		}
		if err != nil {
			return RuleResult{Rule: r, Status: StatusFail, Details: fmt.Sprintf("command exited with error: %v (%s)", err, outputStr)}
		}
		if r.Audit.ExpectedOutput != "" && !outputHasLine(outputStr, r.Audit.ExpectedOutput) {
			return RuleResult{Rule: r, Status: StatusFail, Details: fmt.Sprintf("expected '%s', got '%s'", r.Audit.ExpectedOutput, outputStr)}
		}
		return RuleResult{Rule: r, Status: StatusPass}

	case "cloud_check":
		if run.isRemote() {
			// Cloud checks call provider APIs from this machine; they say nothing about the host.
			return RuleResult{Rule: r, Status: StatusSkip, Details: "cloud check: not evaluated during a remote host scan"}
		}
		chk, ok := cloud.Get(r.Audit.Check)
		if !ok {
			return RuleResult{Rule: r, Status: StatusErr, Details: fmt.Sprintf("unknown cloud check %q (known: %s)", r.Audit.Check, strings.Join(cloud.IDs(), ", "))}
		}
		findings, err := chk.Run(ctx)
		if err != nil {
			return RuleResult{Rule: r, Status: StatusErr, Details: err.Error()}
		}
		if ok, detail := cloud.Summarize(findings); !ok {
			return RuleResult{Rule: r, Status: StatusFail, Details: detail}
		}
		return RuleResult{Rule: r, Status: StatusPass}

	default:
		return RuleResult{Rule: r, Status: StatusSkip, Details: fmt.Sprintf("audit type '%s' evaluated in simulation mode", r.Audit.Type)}
	}
}

// matchContent applies a file_regex/config_regex pattern, honouring must_not_match.
// Configurations exported on Windows use CRLF, which (?m)$ would not match.
func matchContent(r rules.Rule, content []byte, source string) RuleResult {
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	re, err := regexp.Compile(r.Audit.Pattern)
	if err != nil {
		return RuleResult{Rule: r, Status: StatusErr, Details: fmt.Sprintf("invalid regex: %v", err)}
	}
	found := re.Find(content)
	switch {
	case r.Audit.MustNotMatch && found != nil:
		return RuleResult{Rule: r, Status: StatusFail, Details: fmt.Sprintf("forbidden setting %q present in %s", strings.TrimSpace(string(found)), source)}
	case r.Audit.MustNotMatch:
		return RuleResult{Rule: r, Status: StatusPass}
	case found != nil:
		return RuleResult{Rule: r, Status: StatusPass}
	}
	return RuleResult{Rule: r, Status: StatusFail, Details: fmt.Sprintf("pattern '%s' not found in %s", r.Audit.Pattern, source)}
}

var sqlClientRe = regexp.MustCompile(`\b(psql|mysql|mariadb|sqlplus|mongosh|redis-cli|cqlsh)\b`)

// sqlClient names the database client a sql_query rule shells out to, so a host without it
// is reported not applicable instead of failing every control.
func sqlClient(command string) string { return sqlClientRe.FindString(command) }

func anyExists(ctx context.Context, ex Executor, paths []string) bool {
	for _, p := range paths {
		if ex.Exists(ctx, p) {
			return true
		}
	}
	return false
}

// outputHasLine reports whether any trimmed output line equals want exactly.
// Exact matching avoids false passes such as "on" matching inside "command not found".
func outputHasLine(output, want string) bool {
	want = strings.TrimSpace(want)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// FailedRules returns the rules whose audit did not pass, so remediation
// only touches controls that are actually non-compliant.
func (r *ScanReport) FailedRules() []rules.Rule {
	var out []rules.Rule
	for _, res := range r.Results {
		if res.Status == StatusFail || res.Status == StatusErr {
			out = append(out, res.Rule)
		}
	}
	return out
}

// shellCommandWith runs command with the interpreter it was written for. On Windows, POSIX
// rules (cloud CLIs, databases, ...) run under Git for Windows' bash; Linux/macOS run
// PowerShell rules with pwsh when it is installed.
func shellCommandWith(ctx context.Context, command, shell string) *exec.Cmd {
	if shell == "powershell" || (shell == "" && runtime.GOOS == "windows") {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
		}
		return exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", command)
	}
	if shell == "" {
		shell = "sh"
	}
	if runtime.GOOS == "windows" {
		bash := posixShell()
		if bash == "" {
			return exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
				"Write-Output 'this rule needs a POSIX shell: install Git for Windows or set TMAEGIS_BASH'; exit 1")
		}
		return exec.CommandContext(ctx, bash, "-c", command)
	}
	return exec.CommandContext(ctx, shell, "-c", command)
}

// posixShell finds bash on Windows: TMAEGIS_BASH, then Git for Windows. System32\bash.exe is
// the WSL launcher and is never used.
func posixShell() string {
	if p := os.Getenv("TMAEGIS_BASH"); p != "" {
		return p
	}
	var candidates []string
	if git, err := exec.LookPath("git"); err == nil {
		root := filepath.Dir(filepath.Dir(git)) // ...\Git\cmd\git.exe -> ...\Git
		candidates = append(candidates, filepath.Join(root, "bin", "bash.exe"), filepath.Join(root, "usr", "bin", "bash.exe"))
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if base := os.Getenv(env); base != "" {
			candidates = append(candidates, filepath.Join(base, "Git", "bin", "bash.exe"), filepath.Join(base, "Programs", "Git", "bin", "bash.exe"))
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// RunRemediation applies each rule. With dryRun set, it only prints what would change.
func (run *Runner) RunRemediation(ctx context.Context, ruleList []rules.Rule, dryRun bool) error {
	if run.isRemote() {
		// Phase 1 of remote support is strictly read-only.
		return lerr.New("cli.engine.err.remoteRemediation")
	}
	var failed []error
	for _, r := range ruleList {
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Println(i18n.T("cli.engine.remediating", "id", r.ID, "title", r.Title))
		if err := run.remediateRule(ctx, r, dryRun); err != nil {
			// One control that cannot be applied must not roll back every other fix; the
			// re-scan after apply reports it as still failing.
			fmt.Println(i18n.T("cli.engine.remediationSkipped", "id", r.ID, "error", err))
			failed = append(failed, lerr.Wrap("cli.engine.err.remediationFailed", err, "id", r.ID))
		}
	}
	return errors.Join(failed...)
}

func (run *Runner) remediateRule(ctx context.Context, r rules.Rule, dryRun bool) error {
	rem := r.Remediation
	switch rem.Type {
	case "file_line":
		return remediateFileLine(ctx, rem, dryRun)
	case "iac_patch":
		// Cloud/IaC remediation must go through Terraform/Ansible review, never a direct change.
		return run.suggestIaC(ctx, r)
	case "command", "sql_exec":
		if dryRun {
			fmt.Println(i18n.T("cli.engine.dryRun.run", "command", rem.Command))
			return nil
		}
		if out, err := shellCommandWith(ctx, rem.Command, r.Shell()).CombinedOutput(); err != nil {
			return lerr.Wrap("cli.engine.err.commandOutput", err, "output", strings.TrimSpace(string(out)))
		}
		if rem.VerifyCmd != "" {
			if out, err := shellCommandWith(ctx, rem.VerifyCmd, r.Shell()).CombinedOutput(); err != nil {
				return lerr.Wrap("cli.engine.err.verifyFailed", err, "output", strings.TrimSpace(string(out)))
			}
		}
		return nil
	case "":
		return nil
	default:
		return lerr.New("cli.engine.err.unsupportedRemediation", "type", rem.Type)
	}
}

// remediateFileLine rewrites a config file safely: the new content is written to a
// staging file next to the original, validated with VerifyCmd ({file} is replaced by
// the staging path), and only then atomically renamed over the live file.
func remediateFileLine(ctx context.Context, rem rules.RemediateSpec, dryRun bool) error {
	re, err := regexp.Compile(rem.MatchRegex)
	if err != nil {
		return lerr.Wrap("cli.engine.err.invalidMatchRegex", err)
	}
	info, err := os.Stat(rem.FilePath)
	if err != nil {
		return err
	}
	orig, err := os.ReadFile(rem.FilePath)
	if err != nil {
		return err
	}

	updated := ApplyFileLineAt(string(orig), re, rem.ReplaceLine, rem.InsertAt == "top")
	if updated == string(orig) {
		fmt.Println(i18n.T("cli.engine.fileLine.alreadyPresent", "path", rem.FilePath))
		return nil
	}
	if dryRun {
		fmt.Println(i18n.T("cli.engine.dryRun.update", "path", rem.FilePath, "line", strings.TrimSpace(rem.ReplaceLine)))
		return nil
	}

	staging, err := os.CreateTemp(filepath.Dir(rem.FilePath), ".tmaegis-stage-*")
	if err != nil {
		return err
	}
	stagePath := staging.Name()
	defer os.Remove(stagePath)
	if _, err := staging.WriteString(updated); err != nil {
		staging.Close()
		return err
	}
	if err := staging.Close(); err != nil {
		return err
	}
	if err := os.Chmod(stagePath, info.Mode().Perm()); err != nil {
		return err
	}

	if rem.VerifyCmd != "" {
		verify := strings.ReplaceAll(rem.VerifyCmd, "{file}", stagePath)
		if out, err := shellCommandWith(ctx, verify, "").CombinedOutput(); err != nil {
			return lerr.Wrap("cli.engine.err.stagedInvalid", err, "output", strings.TrimSpace(string(out)))
		}
	}
	return os.Rename(stagePath, rem.FilePath)
}

// ApplyFileLine replaces every line matching re with line, or appends line if none matched.
func ApplyFileLine(content string, re *regexp.Regexp, line string) string {
	return ApplyFileLineAt(content, re, line, false)
}

// ApplyFileLineAt is ApplyFileLine with a choice of inserting at the top when nothing matched.
func ApplyFileLineAt(content string, re *regexp.Regexp, line string, top bool) string {
	if top && !re.MatchString(content) {
		return line + "\n" + content
	}
	if re.MatchString(content) {
		first := true
		return re.ReplaceAllStringFunc(content, func(string) string {
			if first {
				first = false
				return line
			}
			return "" // collapse duplicate directives into one
		})
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + line + "\n"
}

// suggestIaC prints (and, with Runner.TerraformDir set, writes) the Terraform that would fix a
// failing cloud_check rule. Nothing is applied: the change belongs in a reviewed pull request.
func (run *Runner) suggestIaC(ctx context.Context, r rules.Rule) error {
	if r.Audit.Type != "cloud_check" {
		fmt.Println(i18n.T("cli.engine.iac.suggested", "command", r.Remediation.Command))
		return nil
	}
	chk, ok := cloud.Get(r.Audit.Check)
	if !ok {
		return lerr.New("cli.engine.err.unknownCloudCheck", "check", r.Audit.Check)
	}
	findings, err := chk.Run(ctx)
	if err != nil {
		return err
	}
	var hcl strings.Builder
	for _, f := range findings {
		if !f.Compliant {
			hcl.WriteString(chk.Terraform(f))
			hcl.WriteString("\n")
		}
	}
	if hcl.Len() == 0 {
		fmt.Println(i18n.T("cli.engine.iac.compliant"))
		return nil
	}
	fmt.Print(i18n.T("cli.engine.iac.proposed") + "\n" + hcl.String())
	if run.TerraformDir != "" {
		if err := os.MkdirAll(run.TerraformDir, 0o755); err != nil {
			return err
		}
		out := filepath.Join(run.TerraformDir, strings.ToLower(strings.ReplaceAll(r.ID, ".", "_"))+".tf")
		if err := os.WriteFile(out, []byte(hcl.String()), 0o644); err != nil {
			return err
		}
		fmt.Println(i18n.T("cli.engine.iac.wrote", "path", out))
	}
	return nil
}
