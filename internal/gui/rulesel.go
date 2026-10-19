// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// The rule-selection logic below mirrors cmd/common.go (selectRules, forThisHost) and
// cmd/scan.go (scopeRules). It is duplicated here, not shared, so the GUI does not depend
// on the CLI package; keep the two in step when platform targets change.

var platformTargets = map[string]bool{
	"linux": true, "windows": true, "macos": true, "solaris": true, "aix": true, "bottlerocket": true, "cos": true,
	"ibm-i": true, "ibm-z": true, "freebsd": true, "intune-macos": true, "intune-ios": true, "azure-container-linux": true, "db2-zos": true, "cics": true, "ios": true, "android": true, "printer": true, "supply-chain": true, "github": true, "gitlab": true, "mcp-server": true, "opnsense": true,
}

var hostOS = map[string]string{
	"iis": "windows", "exchange": "windows", "sharepoint": "windows", "office": "windows",
	"chrome": "windows", "edge": "windows", "firefox": "windows", "zoom": "windows",
	"m365": "windows", "power-platform": "windows", "defender": "windows", "vscode": "windows", "intune-office": "windows", "intune-defender": "windows", "mssql": "", "safari": "macos",
}

func forThisHost(in []rules.Rule, osTarget string) []rules.Rule {
	var out []rules.Rule
	for _, r := range in {
		t := strings.ToLower(r.Target)
		if platformTargets[t] && t != osTarget {
			continue
		}
		if want := hostOS[t]; want != "" && want != osTarget {
			continue
		}
		out = append(out, r)
	}
	return out
}

// scopeRules mirrors cmd/scan.go: device rules (config_regex) only run against a supplied
// device configuration, and when one is supplied only device rules run; manual-review rules
// are listed only on request.
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

// matchFilter is rules.Filter.matches, applied to rules that are already loaded.
func matchFilter(f rules.Filter, r rules.Rule) bool {
	if r.Level > f.MaxLevel {
		return false
	}
	if len(f.Targets) > 0 && !slices.ContainsFunc(f.Targets, func(t string) bool { return strings.EqualFold(t, r.Target) }) {
		return false
	}
	if f.OSFamily != "" && f.OSFamily != "*" && len(r.OSFamily) > 0 {
		return slices.ContainsFunc(r.OSFamily, func(fam string) bool { return strings.EqualFold(fam, f.OSFamily) })
	}
	return true
}

func filterRules(all []rules.Rule, f rules.Filter) []rules.Rule {
	var out []rules.Rule
	for _, r := range all {
		if matchFilter(f, r) {
			out = append(out, r)
		}
	}
	return out
}

// selectRules resolves a scan request into the rules to audit, loading the directory.
func selectRules(p ScanParams, host discovery.Host) ([]rules.Rule, error) {
	all, err := rules.LoadFiltered(p.RulesDir, rules.Filter{MaxLevel: 2})
	if err != nil {
		return nil, err
	}
	return selectFrom(all, p, host)
}

// selectFrom resolves a scan request against rules that are already loaded (every level).
func selectFrom(all []rules.Rule, p ScanParams, host discovery.Host) ([]rules.Rule, error) {
	if p.OSFamily != "" {
		host.Family = p.OSFamily
	}
	level := p.Level
	if level != 2 {
		level = 1
	}
	var loaded []rules.Rule
	switch p.Scope {
	case "auto":
		loaded = filterRules(all, rules.Filter{Targets: host.Targets(), MaxLevel: level, OSFamily: host.Family})
	case "target", "dir":
		f := rules.Filter{MaxLevel: level, OSFamily: host.Family}
		target := strings.TrimSpace(p.Target)
		if p.Scope == "dir" || target == "" || target == "all" {
			loaded = forThisHost(filterRules(all, f), host.Targets()[0])
		} else {
			f.Targets = splitTargets(target)
			loaded = filterRules(all, f)
		}
	default:
		return nil, fmt.Errorf("unknown scope %q", p.Scope)
	}
	return scopeRules(loaded, p.DeviceConfig != "", p.IncludeManual), nil
}

// splitTargets accepts one target or a comma-separated list (the wizard can pick several).
func splitTargets(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
