// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Rule struct {
	ID          string        `yaml:"id" json:"id"`
	Title       string        `yaml:"title" json:"title"`
	Description string        `yaml:"description" json:"description"`
	Target      string        `yaml:"target" json:"target"` // linux, windows, postgres, mysql, mssql, aws, azure, gcp
	OSFamily    []string      `yaml:"os_family,omitempty" json:"os_family,omitempty"`
	Level       int           `yaml:"level" json:"level"`
	Severity    string        `yaml:"severity" json:"severity"`
	Tags        []string      `yaml:"tags,omitempty" json:"tags,omitempty"`
	Reference   string        `yaml:"reference,omitempty" json:"reference,omitempty"` // benchmark and section to verify against
	Audit       AuditSpec     `yaml:"audit" json:"audit"`
	Remediation RemediateSpec `yaml:"remediation" json:"remediation"`
}

type AuditSpec struct {
	Type           string `yaml:"type" json:"type"` // command, file_regex, config_regex, sql_query, registry, cloud_cel, cloud_check, manual
	Command        string `yaml:"command,omitempty" json:"command,omitempty"`
	ExpectedOutput string `yaml:"expected_output,omitempty" json:"expected_output,omitempty"`
	FilePath       string `yaml:"file_path,omitempty" json:"file_path,omitempty"`
	Pattern        string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	// MustNotMatch inverts file_regex/config_regex: the rule passes when Pattern is absent
	// (e.g. "no ip http server" must not be missing, "telnet" must not appear).
	MustNotMatch bool `yaml:"must_not_match,omitempty" json:"must_not_match,omitempty"`
	// Check names a built-in SDK check (see internal/providers/cloud) for audit type cloud_check.
	Check string `yaml:"check,omitempty" json:"check,omitempty"`
	// RequiresFile marks the rule not applicable (SKIP) when none of these paths exist, e.g.
	// control-plane manifests on a Kubernetes worker node.
	RequiresFile []string `yaml:"requires_file,omitempty" json:"requires_file,omitempty"`
	// Shell selects the interpreter on Linux/macOS: "sh" (default) or "bash", which CIS
	// audit scripts require (process substitution, arrays, ${var//x/y}).
	Shell string `yaml:"shell,omitempty" json:"shell,omitempty"`
}

type RemediateSpec struct {
	Type        string `yaml:"type" json:"type"` // command, file_line, sql_exec, iac_patch
	Command     string `yaml:"command,omitempty" json:"command,omitempty"`
	FilePath    string `yaml:"file_path,omitempty" json:"file_path,omitempty"`
	MatchRegex  string `yaml:"match_regex,omitempty" json:"match_regex,omitempty"`
	ReplaceLine string `yaml:"replace_line,omitempty" json:"replace_line,omitempty"`
	VerifyCmd   string `yaml:"verify_cmd,omitempty" json:"verify_cmd,omitempty"`
	// Guidance keeps the benchmark's written remediation next to an executable Command.
	Guidance    string   `yaml:"guidance,omitempty" json:"guidance,omitempty"`
	BackupFiles []string `yaml:"backup_files,omitempty" json:"backup_files,omitempty"`
	// InsertAt controls where file_line adds the line when nothing matches: "bottom" (default)
	// or "top". sshd uses the first value it reads, so sshd rules must insert at the top,
	// before any Include or Match block.
	InsertAt string `yaml:"insert_at,omitempty" json:"insert_at,omitempty"`
}

var (
	auditTypes       = map[string]bool{"command": true, "file_regex": true, "sql_query": true, "registry": true, "cloud_cel": true, "cloud_check": true, "config_regex": true, "manual": true}
	remediationTypes = map[string]bool{"": true, "command": true, "file_line": true, "sql_exec": true, "iac_patch": true}
	severities       = map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
)

// Validate checks that a rule is internally consistent so that malformed
// community rules fail at load time instead of at remediation time.
func (r *Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rule has no id")
	}
	if r.Title == "" {
		return fmt.Errorf("%s: missing title", r.ID)
	}
	if r.Target == "" {
		return fmt.Errorf("%s: missing target", r.ID)
	}
	if r.Level < 1 || r.Level > 2 {
		return fmt.Errorf("%s: level must be 1 or 2, got %d", r.ID, r.Level)
	}
	if !severities[strings.ToLower(r.Severity)] {
		return fmt.Errorf("%s: invalid severity %q", r.ID, r.Severity)
	}
	if !auditTypes[r.Audit.Type] {
		return fmt.Errorf("%s: unsupported audit type %q", r.ID, r.Audit.Type)
	}
	if r.Audit.Shell != "" && r.Audit.Shell != "sh" && r.Audit.Shell != "bash" && r.Audit.Shell != "powershell" {
		return fmt.Errorf("%s: audit shell must be sh, bash or powershell, got %q", r.ID, r.Audit.Shell)
	}
	switch r.Audit.Type {
	case "file_regex":
		if r.Audit.FilePath == "" || r.Audit.Pattern == "" {
			return fmt.Errorf("%s: file_regex audit needs file_path and pattern", r.ID)
		}
		if _, err := regexp.Compile(r.Audit.Pattern); err != nil {
			return fmt.Errorf("%s: invalid audit pattern: %w", r.ID, err)
		}
	case "config_regex":
		if r.Audit.Pattern == "" {
			return fmt.Errorf("%s: config_regex audit needs a pattern", r.ID)
		}
		if _, err := regexp.Compile(r.Audit.Pattern); err != nil {
			return fmt.Errorf("%s: invalid audit pattern: %w", r.ID, err)
		}
	case "manual":
		if r.Audit.Command == "" {
			return fmt.Errorf("%s: manual audit needs review instructions in command", r.ID)
		}
	case "cloud_check":
		if r.Audit.Check == "" {
			return fmt.Errorf("%s: cloud_check audit needs a check name", r.ID)
		}
	default:
		if r.Audit.Command == "" {
			return fmt.Errorf("%s: %s audit needs a command", r.ID, r.Audit.Type)
		}
	}
	if !remediationTypes[r.Remediation.Type] {
		return fmt.Errorf("%s: unsupported remediation type %q", r.ID, r.Remediation.Type)
	}
	switch r.Remediation.Type {
	case "file_line":
		if r.Remediation.FilePath == "" || r.Remediation.MatchRegex == "" || r.Remediation.ReplaceLine == "" {
			return fmt.Errorf("%s: file_line remediation needs file_path, match_regex and replace_line", r.ID)
		}
		if _, err := regexp.Compile(r.Remediation.MatchRegex); err != nil {
			return fmt.Errorf("%s: invalid match_regex: %w", r.ID, err)
		}
		if ia := r.Remediation.InsertAt; ia != "" && ia != "top" && ia != "bottom" {
			return fmt.Errorf("%s: insert_at must be top or bottom, got %q", r.ID, ia)
		}
	case "command", "sql_exec", "iac_patch":
		if r.Remediation.Command == "" && !(r.Remediation.Type == "iac_patch" && r.Audit.Type == "cloud_check") {
			return fmt.Errorf("%s: %s remediation needs a command", r.ID, r.Remediation.Type)
		}
	}
	return nil
}

// powershellTargets are products whose rules are written in PowerShell (Windows itself,
// Windows-only servers and registry-policy based desktop software).
var powershellTargets = map[string]bool{
	"windows": true, "iis": true, "exchange": true, "sharepoint": true, "office": true, "chrome": true,
	"edge": true, "firefox": true, "zoom": true, "m365": true, "power-platform": true, "mssql": true,
	"defender": true, "vscode": true, "intune-office": true, "intune-defender": true,
}

// Shell returns the interpreter the rule's commands are written for: the explicit audit
// shell, PowerShell for Windows-native targets, otherwise POSIX sh.
func (r Rule) Shell() string {
	if r.Audit.Shell != "" {
		return r.Audit.Shell
	}
	if powershellTargets[strings.ToLower(r.Target)] {
		return "powershell"
	}
	return "sh"
}

// Filter selects which rules to load. Empty Targets means all targets.
type Filter struct {
	Targets  []string // rule targets to include (case-insensitive)
	MaxLevel int      // CIS profile level ceiling
	// OSFamily, when set (e.g. "ubuntu-22.04"), drops rules that declare an
	// os_family list not containing it. Rules without os_family always apply.
	OSFamily string
}

func (f Filter) matches(r Rule) bool {
	if r.Level > f.MaxLevel {
		return false
	}
	if len(f.Targets) > 0 {
		ok := false
		for _, t := range f.Targets {
			if strings.EqualFold(t, r.Target) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if f.OSFamily != "" && f.OSFamily != "*" && len(r.OSFamily) > 0 {
		for _, fam := range r.OSFamily {
			if strings.EqualFold(fam, f.OSFamily) {
				return true
			}
		}
		return false
	}
	return true
}

func LoadFromDir(dir string, targetFilter string, maxLevel int) ([]Rule, error) {
	f := Filter{MaxLevel: maxLevel}
	if targetFilter != "all" {
		f.Targets = []string{targetFilter}
	}
	return LoadFiltered(dir, f)
}

// LoadFiltered loads and validates every rule under dir, returning those matching f.
func LoadFiltered(dir string, f Filter) ([]Rule, error) {
	var loaded []Rule
	seen := make(map[string]bool)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		var r Rule
		if err := yaml.Unmarshal(data, &r); err != nil {
			return fmt.Errorf("parsing YAML in %s: %w", path, err)
		}

		if r.ID == "" {
			return fmt.Errorf("%s: rule has no id", path)
		}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if seen[r.ID] {
			return fmt.Errorf("%s: duplicate rule id %s", path, r.ID)
		}
		seen[r.ID] = true
		if !f.matches(r) {
			return nil
		}

		loaded = append(loaded, r)
		return nil
	})

	sort.Slice(loaded, func(i, j int) bool { return loaded[i].ID < loaded[j].ID })
	return loaded, err
}
