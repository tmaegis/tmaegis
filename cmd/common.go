// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"github.com/tmaegis/tmaegis/internal/i18n"
	"os"
	"strings"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// selectRules loads rules for an explicit target or, with autoDetect, for everything
// discovered on this host (OS + installed software, filtered by distro version).
func selectRules(target string, autoDetect bool, level int) ([]rules.Rule, error) {
	rs, _, err := selectRulesHost(target, autoDetect, level)
	return rs, err
}

// selectRulesHost is selectRules that also returns the detected host.
func selectRulesHost(target string, autoDetect bool, level int) ([]rules.Rule, discovery.Host, error) {
	host := discovery.Detect()
	if osFamilyOverride != "" {
		host.Family = osFamilyOverride
	}
	if !autoDetect {
		// Rules scoped to other distro releases (os_family) never apply to this host.
		f := rules.Filter{MaxLevel: level, OSFamily: host.Family}
		if target != "all" {
			f.Targets = []string{target}
			rs, err := rules.LoadFiltered(rulesDir, f)
			return rs, host, err
		}
		all, err := rules.LoadFiltered(rulesDir, f)
		if err != nil {
			return nil, host, err
		}
		return forThisHost(all, host.Targets()[0]), host, nil
	}
	say(os.Stderr, i18n.T("cli.select.autoDetected",
		"os", host.OS, "family", host.Family, "software", strings.Join(host.Software, ", ")))
	rs, err := rules.LoadFiltered(rulesDir, rules.Filter{Targets: host.Targets(), MaxLevel: level, OSFamily: host.Family})
	return rs, host, err
}

// platformTargets are operating systems and devices: "all" on one machine must not run another
// platform's rules. supply-chain audits a git checkout, so it only runs when asked for by name.
var platformTargets = map[string]bool{
	"linux": true, "windows": true, "macos": true, "solaris": true, "aix": true, "bottlerocket": true, "cos": true,
	"ibm-i": true, "ibm-z": true, "freebsd": true, "intune-macos": true, "intune-ios": true, "azure-container-linux": true, "db2-zos": true, "cics": true, "ios": true, "android": true, "printer": true, "supply-chain": true, "github": true, "gitlab": true, "mcp-server": true, "opnsense": true,
}

// hostOS pins software whose audits only run on one OS (PowerShell or registry policies,
// macOS defaults).
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

func newRunner() *engine.Runner {
	return engine.NewRunnerWithConcurrency(cfg.Engine.Concurrency)
}
