// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// The Markdown report is plain GitHub-flavoured Markdown: headings, bullet lists, pipe tables and
// fenced code blocks only. It has no HTML (no <details>), no emoji-only status and no extension
// syntax, so it pastes into GitHub issues, PRs and wikis, Confluence and Notion, and still reads
// well as plain text. Every string that comes from a rule or a scanned host is untrusted: inline
// text goes through mdEscape, multi-line evidence goes into a fenced block whose fence is longer
// than any backtick run inside it.

const (
	mdEvidenceMax    = 4000 // runes of scan output kept per finding (the HTML report keeps the same)
	mdInstructionMax = 3000 // runes of manual-review instructions
	mdTopBenchmarks  = 5
)

// mdSanitize drops control characters (except newline and tab) and Unicode bidirectional
// overrides, which could reorder the text around them, and normalises line breaks to "\n".
func mdSanitize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20 || r == 0x7f || r >= 0x80 && r < 0xa0:
			return -1
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0x200e, r == 0x200f, r == 0x061c, r == 0xfeff:
			return -1
		}
		return r
	}, s)
}

// mdEscape makes s safe as inline Markdown text on one line (also inside a table cell): Markdown
// punctuation is backslash-escaped, < and > become entities so no HTML can be injected, and
// anything that would start a block (#, -, +, a number and a dot) is escaped at the start.
func mdEscape(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(mdSanitize(s), "\t", " ")), " ")
	var b strings.Builder
	for i, r := range s {
		switch r {
		case '\\', '`', '*', '_', '[', ']', '|', '~', '&':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '#', '-', '+', '=':
			if i == 0 {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		case '.', ')':
			// "1. x" and "1) x" start an ordered list
			if i > 0 && allDigits(s[:i]) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// mdFence wraps text in a fenced code block. The fence is one backtick longer than the longest
// run of backticks in the text (at least three), so the content cannot close it early.
func mdFence(text string) string {
	text = strings.TrimRight(mdSanitize(text), "\n ")
	longest, run := 0, 0
	for _, r := range text {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	return fence + "\n" + text + "\n" + fence + "\n"
}

// mdCode renders a short single-line value as an inline code span, choosing a delimiter that does
// not occur inside it.
func mdCode(s string) string {
	s = strings.Join(strings.Fields(mdSanitize(s)), " ")
	if s == "" {
		return ""
	}
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	d := strings.Repeat("`", longest+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return d + s + d
}

type mdFinding struct {
	r        engine.RuleResult
	class    string
	status   string
	rem      string
	guidance bool
}

type mdCount struct{ pass, fail, skip, manual int }

func (c mdCount) total() int { return c.pass + c.fail + c.skip + c.manual }

func writeMarkdown(w io.Writer, rep *engine.ScanReport, c *ctx) error {
	tr := c.tr
	bw := bufio.NewWriter(w)
	p := func(format string, a ...any) { fmt.Fprintf(bw, format, a...) }

	sevOrder := []string{"critical", "high", "medium", "low", "other"}
	counts := map[string]*mdCount{}
	for _, k := range sevOrder {
		counts[k] = &mdCount{}
	}
	failed := map[string][]mdFinding{}
	var manual, na []engine.RuleResult
	var all mdCount
	type bench struct {
		name       string
		pass, fail int
	}
	benches := map[string]*bench{}
	targets := map[string]bool{}
	levels := map[int]bool{}

	for _, r := range rep.Results {
		cl := sevClass(r.Rule.Severity)
		ct := counts[cl]
		bk := benchKey(r.Rule.ID, r.Rule.Target)
		if bk == "" {
			bk = "-"
		}
		b := benches[bk]
		if b == nil {
			b = &bench{name: bk}
			if bk == "-" {
				b.name = tr.T("report.bench.other")
			}
			benches[bk] = b
		}
		if r.Rule.Target != "" {
			targets[r.Rule.Target] = true
		}
		levels[r.Rule.Level] = true
		switch strings.ToUpper(string(r.Status)) {
		case "PASS":
			ct.pass++
			all.pass++
			b.pass++
		case "SKIP":
			if r.Rule.Audit.Type == "manual" {
				ct.manual++
				all.manual++
				manual = append(manual, r)
			} else {
				ct.skip++
				all.skip++
				na = append(na, r)
			}
		default:
			f := mdFinding{r: r, class: cl, status: tr.T("report.status.fail")}
			if strings.ToUpper(string(r.Status)) == "ERROR" {
				f.status = tr.T("report.status.error")
			}
			f.rem, f.guidance = remediationText(r)
			ct.fail++
			all.fail++
			b.fail++
			failed[cl] = append(failed[cl], f)
		}
	}

	// ---- title and metadata
	p("# %s\n\n", mdEscape(tr.T("report.app.title")))
	meta := func(key, val string) {
		if strings.TrimSpace(val) != "" {
			p("- **%s:** %s\n", mdEscape(tr.T(key)), val)
		}
	}
	host := rep.Hostname
	if strings.TrimSpace(host) == "" {
		host = tr.T("report.host.unknown")
	}
	meta("report.host.hostname", mdEscape(host))
	osText := mdEscape(rep.OS)
	if rep.Arch != "" {
		osText += " (" + mdEscape(rep.Arch) + ")"
	}
	meta("report.host.os", osText)
	tl := make([]string, 0, len(targets))
	for t := range targets {
		tl = append(tl, mdEscape(t))
	}
	sort.Strings(tl)
	meta("report.host.targets", strings.Join(tl, ", "))
	meta("report.host.rulesdir", mdCode(c.info.RulesDir))
	var lvl []int
	for l := range levels {
		lvl = append(lvl, l)
	}
	sort.Ints(lvl)
	var lt []string
	for _, l := range lvl {
		lt = append(lt, fmt.Sprintf("L%d", l))
	}
	meta("report.about.options.levels", strings.Join(lt, ", "))
	when := rep.Timestamp
	if when.IsZero() {
		when = time.Now()
	}
	meta("report.host.scanned", when.UTC().Format("2006-01-02 15:04 UTC"))
	if c.info.Duration > 0 {
		meta("report.host.duration", mdEscape(c.info.Duration.Round(time.Millisecond).String()))
	}
	if len(c.info.Options) > 0 {
		opts := make([]string, len(c.info.Options))
		for i, o := range c.info.Options {
			opts[i] = mdCode(o)
		}
		meta("report.about.options.extra", strings.Join(opts, " "))
	}
	version := c.info.Version
	if version == "" {
		version = "dev"
	}
	meta("report.about.tool.version", mdEscape("TMAegis "+version))
	p("\n")

	// ---- summary
	sc := rep.Score()
	evaluated := all.pass + all.fail
	verdict := tr.T("report.verdict.good")
	switch {
	case sc < 70:
		verdict = tr.T("report.verdict.poor")
	case sc < 90:
		verdict = tr.T("report.verdict.fair")
	}
	p("## %s\n\n", mdEscape(tr.T("report.pdf.summary")))
	if evaluated > 0 {
		p("**%s:** %.1f%% - %s\n\n", mdEscape(tr.T("report.score.label")), sc, mdEscape(verdict))
		if all.fail > 0 {
			urgent := len(failed["critical"]) + len(failed["high"])
			p("%s\n\n", mdEscape(tr.N("report.lead.some", all.fail, "evaluated", evaluated, "urgent", urgent)))
		} else {
			p("%s\n\n", mdEscape(tr.T("report.lead.allpass")))
		}
	} else {
		p("%s\n\n", mdEscape(tr.T("report.lead.none")))
	}
	p("| %s | %s | %s | %s | %s | %s |\n", mdEscape(tr.T("report.col.severity")), mdEscape(tr.T("report.stat.passed")),
		mdEscape(tr.T("report.stat.failed")), mdEscape(tr.T("report.stat.skipped")), mdEscape(tr.T("report.about.counts.manual")),
		mdEscape(tr.T("report.stat.total")))
	p("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, k := range sevOrder {
		ct := counts[k]
		if k == "other" && ct.total() == 0 {
			continue
		}
		p("| %s | %d | %d | %d | %d | %d |\n", mdEscape(tr.T("report.sev."+k)), ct.pass, ct.fail, ct.skip, ct.manual, ct.total())
	}
	p("| **%s** | **%d** | **%d** | **%d** | **%d** | **%d** |\n\n", mdEscape(tr.T("report.stat.total")), all.pass, all.fail, all.skip, all.manual, all.total())

	// ---- top failing categories
	var bl []*bench
	for _, b := range benches {
		if b.fail > 0 {
			bl = append(bl, b)
		}
	}
	sort.Slice(bl, func(i, j int) bool {
		if bl[i].fail != bl[j].fail {
			return bl[i].fail > bl[j].fail
		}
		return bl[i].name < bl[j].name
	})
	if len(bl) > mdTopBenchmarks {
		bl = bl[:mdTopBenchmarks]
	}
	if len(bl) > 0 {
		p("## %s\n\n", mdEscape(tr.T("report.bench.title")))
		p("| %s | %s | %s | %s |\n| --- | ---: | ---: | ---: |\n", mdEscape(tr.T("report.results.benchmark")),
			mdEscape(tr.T("report.stat.failed")), mdEscape(tr.T("report.stat.passed")), mdEscape(tr.T("report.bench.score")))
		for _, b := range bl {
			p("| %s | %d | %d | %.1f%% |\n", mdEscape(b.name), b.fail, b.pass, float64(b.pass)*100/float64(b.pass+b.fail))
		}
		p("\n")
	}

	// ---- findings by severity
	p("## %s\n\n", mdEscape(tr.T("report.sev.title")))
	if all.fail == 0 {
		p("%s\n\n", mdEscape(tr.T("report.sev.none")))
	}
	for _, k := range sevOrder {
		fs := failed[k]
		if len(fs) == 0 {
			continue
		}
		p("### %s (%d)\n\n", mdEscape(tr.T("report.sev."+k)), len(fs))
		for _, f := range fs {
			r := f.r
			p("#### %s - %s\n\n", mdEscape(r.Rule.ID), mdEscape(r.Rule.Title))
			p("- **%s:** %s\n", mdEscape(tr.T("report.col.status")), mdEscape(f.status))
			p("- **%s:** %s\n", mdEscape(tr.T("report.col.severity")), mdEscape(tr.T("report.sev."+k)))
			if r.Rule.Target != "" {
				p("- **%s:** %s\n", mdEscape(tr.T("report.results.target")), mdEscape(r.Rule.Target))
			}
			if ref := strings.TrimSpace(r.Rule.Reference); ref != "" {
				p("- **%s:** %s\n", mdEscape(tr.T("report.detail.reference")), mdEscape(ref))
			}
			p("\n")
			if d := strings.TrimSpace(mdSanitize(r.Details)); d != "" {
				p("**%s**\n\n%s\n", mdEscape(tr.T("report.detail.output")), mdFence(clip(d, mdEvidenceMax)))
			}
			if strings.TrimSpace(f.rem) != "" {
				label := mdEscape(tr.T("report.detail.remediation"))
				if f.guidance {
					label += " (" + mdEscape(tr.T("report.risk.guidance")) + ")"
				}
				p("**%s**\n\n%s\n", label, mdFence(f.rem))
			} else {
				p("**%s:** %s\n\n", mdEscape(tr.T("report.detail.remediation")), mdEscape(tr.T("report.plan.noremediation")))
			}
		}
	}

	// ---- manual review
	if len(manual) > 0 {
		p("## %s\n\n%s\n\n", mdEscape(tr.T("report.skipped.manual.title")),
			mdEscape(tr.N("report.skipped.manual.count", len(manual))))
		for _, r := range manual {
			p("### %s - %s\n\n", mdEscape(r.Rule.ID), mdEscape(r.Rule.Title))
			p("- **%s:** %s\n", mdEscape(tr.T("report.col.severity")), mdEscape(tr.T("report.sev."+sevClass(r.Rule.Severity))))
			if r.Rule.Target != "" {
				p("- **%s:** %s\n", mdEscape(tr.T("report.results.target")), mdEscape(r.Rule.Target))
			}
			if ref := strings.TrimSpace(r.Rule.Reference); ref != "" {
				p("- **%s:** %s\n", mdEscape(tr.T("report.detail.reference")), mdEscape(ref))
			}
			p("\n")
			ins := strings.TrimSpace(r.Rule.Audit.Command)
			if ins == "" {
				ins = strings.TrimPrefix(r.Details, "manual review: ")
			}
			if ins = strings.TrimSpace(ins); ins != "" {
				p("**%s**\n\n%s\n", mdEscape(tr.T("report.skipped.instructions")), mdFence(clip(ins, mdInstructionMax)))
			}
		}
	}

	// ---- not applicable
	if len(na) > 0 {
		p("## %s\n\n%s\n\n", mdEscape(tr.T("report.skipped.na.title")), mdEscape(tr.N("report.skipped.na.count", len(na))))
		for _, r := range na {
			p("- %s - %s\n", mdEscape(r.Rule.ID), mdEscape(r.Rule.Title))
		}
		p("\n")
	}

	// ---- footer
	p("---\n\n")
	p("%s\n\n", mdEscape(tr.T("report.pdf.footer")))
	p("%s %s\n", mdEscape(tr.T("report.about.legal.tm")), mdEscape(tr.T("report.about.licences.tmaegis")))
	return bw.Flush()
}
