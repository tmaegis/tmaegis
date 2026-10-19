// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"os"
	"regexp"
	"strings"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// DiffLine is one line of a preview diff: Op is " " (context), "-" (removed) or "+" (added).
type DiffLine struct {
	Op   string `json:"op"`
	Text string `json:"text"`
	Old  int    `json:"old,omitempty"` // 1-based line number in the current file
	New  int    `json:"new,omitempty"` // 1-based line number after the change
}

// Diff is the preview of a file_line change. Error is a reason code when the current file
// could not be read (the change itself is still shown as an added line).
type Diff struct {
	File      string     `json:"file"`
	Lines     []DiffLine `json:"lines"`
	Unchanged bool       `json:"unchanged,omitempty"`
	Truncated bool       `json:"truncated,omitempty"`
	Error     string     `json:"error,omitempty"` // unreadable, bad_regex
}

const (
	diffContext  = 3
	diffMaxLines = 80
	diffMaxBytes = 2 << 20
)

// fileLineDiff computes what a file_line remediation would do to the file, read-only. It
// uses the engine's own rewrite function so the preview cannot drift from the real change.
func fileLineDiff(rem rules.RemediateSpec) *Diff {
	d := &Diff{File: rem.FilePath}
	re, err := regexp.Compile(rem.MatchRegex)
	if err != nil {
		d.Error = "bad_regex"
		return d
	}
	b, err := os.ReadFile(rem.FilePath)
	if err != nil || len(b) > diffMaxBytes {
		d.Error = "unreadable"
		d.Lines = []DiffLine{{Op: "+", Text: strings.TrimRight(rem.ReplaceLine, "\n")}}
		return d
	}
	orig := string(b)
	updated := engine.ApplyFileLineAt(orig, re, rem.ReplaceLine, rem.InsertAt == "top")
	if updated == orig {
		d.Unchanged = true
		return d
	}
	d.Lines, d.Truncated = lineDiff(splitLines(orig), splitLines(updated))
	return d
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// lineDiff is a small LCS diff with context, good for configuration files (bounded size).
func lineDiff(a, b []string) ([]DiffLine, bool) {
	// trim the common prefix and suffix first, so the quadratic part stays tiny
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	ma, mb := a[pre:len(a)-suf], b[pre:len(b)-suf]
	var mid []DiffLine
	if len(ma)*len(mb) > 250000 { // too big for LCS: show it as a replacement
		for _, l := range ma {
			mid = append(mid, DiffLine{Op: "-", Text: l})
		}
		for _, l := range mb {
			mid = append(mid, DiffLine{Op: "+", Text: l})
		}
	} else {
		lcs := make([][]int, len(ma)+1)
		for i := range lcs {
			lcs[i] = make([]int, len(mb)+1)
		}
		for i := len(ma) - 1; i >= 0; i-- {
			for j := len(mb) - 1; j >= 0; j-- {
				if ma[i] == mb[j] {
					lcs[i][j] = lcs[i+1][j+1] + 1
				} else {
					lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
				}
			}
		}
		i, j := 0, 0
		for i < len(ma) || j < len(mb) {
			switch {
			case i < len(ma) && j < len(mb) && ma[i] == mb[j]:
				mid = append(mid, DiffLine{Op: " ", Text: ma[i]})
				i++
				j++
			case i < len(ma) && (j == len(mb) || lcs[i+1][j] >= lcs[i][j+1]):
				mid = append(mid, DiffLine{Op: "-", Text: ma[i]}) // removals before additions
				i++
			default:
				mid = append(mid, DiffLine{Op: "+", Text: mb[j]})
				j++
			}
		}
	}
	// assemble with line numbers, keeping diffContext lines around each change
	all := make([]DiffLine, 0, len(a)+len(mb))
	for k := 0; k < pre; k++ {
		all = append(all, DiffLine{Op: " ", Text: a[k]})
	}
	all = append(all, mid...)
	for k := len(a) - suf; k < len(a); k++ {
		all = append(all, DiffLine{Op: " ", Text: a[k]})
	}
	o, n := 1, 1
	for k := range all {
		switch all[k].Op {
		case " ":
			all[k].Old, all[k].New = o, n
			o++
			n++
		case "-":
			all[k].Old = o
			o++
		case "+":
			all[k].New = n
			n++
		}
	}
	keep := make([]bool, len(all))
	for k, l := range all {
		if l.Op == " " {
			continue
		}
		for c := max(0, k-diffContext); c <= min(len(all)-1, k+diffContext); c++ {
			keep[c] = true
		}
	}
	var out []DiffLine
	last := -1
	for k, l := range all {
		if !keep[k] {
			continue
		}
		if last >= 0 && k != last+1 {
			out = append(out, DiffLine{Op: "@", Text: ""}) // gap between hunks
		}
		out = append(out, l)
		last = k
	}
	if len(out) > diffMaxLines {
		return out[:diffMaxLines], true
	}
	return out, false
}

var (
	highRiskRe   = regexp.MustCompile(`(?i)\b(systemctl|service|reboot|shutdown|restart|iptables|nft|ufw|firewall-cmd|netsh|sshd\w*|pam\w*|passwd|usermod|userdel|chpasswd|auditctl|grub|mount|fstab|modprobe|rm\s+-rf|Disable-|Set-NetFirewall|bcdedit|kernel)\b`)
	mediumRiskRe = regexp.MustCompile(`(?i)\b(chmod|chown|sysctl|reg(\.exe)?\s+add|Set-ItemProperty|New-ItemProperty|secedit|auditpol|gpupdate|ALTER|GRANT|REVOKE|UPDATE)\b`)
)

// riskOf rates how disruptive a remediation could be. It is a heuristic shown to help the
// operator review, not a guarantee: the dry run, snapshot and dead-man timer are the safety net.
// Reasons are gui.fix.reason_* message suffixes.
func riskOf(r rules.Rule) (string, []string) {
	rem := r.Remediation
	var reasons []string
	risk := "low"
	bump := func(to string) {
		if to == "high" || (to == "medium" && risk == "low") {
			risk = to
		}
	}
	switch rem.Type {
	case "iac_patch":
		return "low", []string{"suggestion"}
	case "file_line":
		reasons = append(reasons, "single_line")
		if rem.VerifyCmd != "" {
			reasons = append(reasons, "verified")
		} else {
			bump("medium")
			reasons = append(reasons, "unverified")
		}
	case "sql_exec":
		bump("medium")
		reasons = append(reasons, "database")
	case "command":
		bump("medium")
		reasons = append(reasons, "command")
	}
	text := rem.Command + "\n" + rem.FilePath
	if highRiskRe.MatchString(text) {
		bump("high")
		reasons = append(reasons, "service_or_access")
	} else if mediumRiskRe.MatchString(text) {
		bump("medium")
		reasons = append(reasons, "permissions_or_policy")
	}
	if strings.EqualFold(r.Severity, "critical") || strings.EqualFold(r.Severity, "high") {
		reasons = append(reasons, "important_control")
	}
	return risk, reasons
}
