// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/ui"
)

// Terminal styling. Every styled path is taken only when the stream is a terminal (or
// FORCE_COLOR is set); otherwise the CLI prints exactly what it always printed, so pipes,
// files, report formats and scripts see no escape sequences or glyphs.

var (
	noColorFlag bool
	asciiFlag   bool
	// rawOpts are --no-color/--ascii read from the raw arguments, before cobra parses them
	// (--help and --lang list are rendered that early).
	rawOpts ui.Options
)

func uiOptions() ui.Options {
	return ui.Options{NoColor: noColorFlag || rawOpts.NoColor, ASCII: asciiFlag || rawOpts.ASCII}
}

// optionsFromArgs finds --no-color and --ascii in raw arguments (parsing stops at "--").
func optionsFromArgs(args []string) ui.Options {
	var o ui.Options
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "--no-color", "--no-color=true":
			o.NoColor = true
		case "--ascii", "--ascii=true":
			o.ASCII = true
		}
	}
	return o
}

// printerFor styles w when it is a terminal and is plain for anything else.
func printerFor(w io.Writer) *ui.Printer { return ui.New(w, ui.For(w, uiOptions())) }

func outUI() *ui.Printer { return printerFor(os.Stdout) }
func errUI() *ui.Printer { return printerFor(os.Stderr) }

// summaryFollows is set while a local scan will print the summary card, which lists the
// written report itself (so writeReport skips its one-line notice on a terminal).
var summaryFollows bool

// say prints a localized message that starts with a legacy marker, styled on a terminal.
func say(w io.Writer, msg string) {
	p := printerFor(w)
	p.Println(p.Line(msg))
}

func bannerInfo() ui.BannerInfo {
	return ui.BannerInfo{Name: "TMAegis", Version: Version, Tagline: i18n.T("cli.banner.tagline"), Subtitle: i18n.T("cli.banner.subtitle"), Beta: i18n.T("cli.banner.beta")}
}

// code wraps a command line in backticks, which terminals highlight.
func code(s string) string { return "`" + s + "`" }

// ---------------------------------------------------------------- errors

// hintedError adds a fix hint to an error without changing its message.
type hintedError struct {
	err  error
	hint string
}

func (h *hintedError) Error() string { return h.err.Error() }
func (h *hintedError) Unwrap() error { return h.err }

func withHint(err error, hint string) error {
	if err == nil {
		return nil
	}
	return &hintedError{err: err, hint: hint}
}

func rulesDirHint() string {
	return i18n.T("cli.ui.hint.rulesDir", "dir", rulesDir, "command", code("tmaegis rules validate -r "+rulesDir))
}

// hintFor picks a fix hint for err (cmdPath is the command that failed).
func hintFor(err error, cmdPath string) string {
	var h *hintedError
	if errors.As(err, &h) {
		return h.hint
	}
	msg := err.Error()
	for _, s := range []string{"unknown flag", "unknown shorthand flag", "unknown command", "flag needs an argument", "invalid argument", "accepts ", "requires "} {
		if strings.Contains(msg, s) {
			return i18n.T("cli.ui.hint.help", "command", code(cmdPath+" --help"))
		}
	}
	return ""
}

// printError reports a failed command on stderr: "Error: <message>" and, when one is known,
// a hint on how to fix it.
func printError(err error, cmdPath string) {
	p := errUI()
	p.Print(p.Problem(true, i18n.T("cli.ui.error", "message", err.Error()), hintFor(err, cmdPath)))
}

// ---------------------------------------------------------------- setup panel

// benchmarks lists the distinct benchmark names the rules reference (most common first).
func benchmarks(rs []rules.Rule) []string {
	count := map[string]int{}
	for _, r := range rs {
		ref := strings.TrimSpace(r.Reference)
		if ref == "" {
			continue
		}
		if i := strings.Index(ref, " - "); i > 0 {
			ref = ref[:i]
		}
		count[ref]++
	}
	out := make([]string, 0, len(count))
	for k := range count {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if count[out[i]] != count[out[j]] {
			return count[out[i]] > count[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func hostValue(h discovery.Host) string {
	parts := []string{h.OS}
	if d := strings.TrimSpace(h.Distro + " " + h.Version); d != "" {
		parts = append(parts, d)
	}
	return strings.Join(parts, " ")
}

// setupPanel describes what is about to be audited.
func setupPanel(p *ui.Printer, title, target string, level int, host discovery.Host, rs []rules.Rule, extra ...ui.KV) string {
	family := host.Family
	if family == "" {
		family = "-"
	}
	rows := []ui.KV{
		{Key: i18n.T("cli.ui.kv.target"), Value: p.Bold(target)},
		{Key: i18n.T("cli.ui.kv.host"), Value: hostValue(host)},
		{Key: i18n.T("cli.ui.kv.osFamily"), Value: family},
		{Key: i18n.T("cli.ui.kv.level"), Value: fmt.Sprint(level)},
		{Key: i18n.T("cli.ui.kv.rulesDir"), Value: rulesDir},
		{Key: i18n.T("cli.ui.kv.rules"), Value: p.Accent(fmt.Sprint(len(rs)))},
	}
	if b := benchmarks(rs); len(b) > 0 {
		v := b[0]
		if len(b) > 1 {
			v = i18n.N("cli.ui.kv.benchmarksMore", len(b)-1, "first", b[0])
		}
		rows = append(rows, ui.KV{Key: i18n.T("cli.ui.kv.benchmarks"), Value: v})
	}
	rows = append(rows, extra...)
	return p.Section(title) + "\n" + p.Panel(rows)
}

// auditWithProgress runs the audit with a live progress line on stderr.
func auditWithProgress(run func(progress func(rules.Rule)) *engine.ScanReport, total int) *engine.ScanReport {
	prog := ui.NewProgress(errUI(), total, i18n.T("cli.ui.progress.auditing"), func(done, total, pct int) string {
		return i18n.T("cli.scan.progress", "done", done, "total", total, "percent", pct)
	})
	rep := run(func(r rules.Rule) { prog.Inc(r.ID) })
	prog.Done()
	return rep
}

// ---------------------------------------------------------------- results

type scanCounts struct{ passed, failed, errs, skipped, manual int }

func countResults(r *engine.ScanReport) scanCounts {
	var c scanCounts
	for _, res := range r.Results {
		switch res.Status {
		case engine.StatusPass:
			c.passed++
		case engine.StatusFail:
			c.failed++
		case engine.StatusErr:
			c.errs++
		case engine.StatusSkip:
			if res.Rule.Audit.Type == "manual" {
				c.manual++
			} else {
				c.skipped++
			}
		}
	}
	return c
}

var severityRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

func sevRank(s string) int {
	if r, ok := severityRank[strings.ToLower(s)]; ok {
		return r
	}
	return 4
}

// category is the most specific tag that is not the platform itself.
func category(r rules.Rule) string {
	for i := len(r.Tags) - 1; i >= 0; i-- {
		t := strings.TrimSpace(r.Tags[i])
		if t != "" && !strings.EqualFold(t, r.Target) && !platformTargets[strings.ToLower(t)] {
			return t
		}
	}
	return "other"
}

// writeResultsTable renders the findings: passed, then skipped/manual, then failed (most
// severe first) so the failures sit right above the summary card.
func writeResultsTable(p *ui.Printer, r *engine.ScanReport) {
	var pass, skip, fail []engine.RuleResult
	for _, res := range r.Results {
		switch res.Status {
		case engine.StatusPass:
			pass = append(pass, res)
		case engine.StatusSkip:
			skip = append(skip, res)
		default:
			fail = append(fail, res)
		}
	}
	sort.SliceStable(fail, func(i, j int) bool { return sevRank(fail[i].Rule.Severity) < sevRank(fail[j].Rule.Severity) })
	idW, titleW := 0, 0
	for _, res := range r.Results {
		idW = max(idW, ui.Width(res.Rule.ID))
		titleW = max(titleW, ui.Width(res.Rule.Title))
	}
	cols := []ui.Column{
		{Title: i18n.T("cli.ui.col.status"), Width: ui.Width(p.Status("MANUAL", false))},
		{Title: i18n.T("cli.ui.col.severity"), Width: ui.ChipWidth},
		{Title: i18n.T("cli.ui.col.rule"), Flex: true, Min: 12, Width: idW},
		{Title: i18n.T("cli.ui.col.title"), Flex: true, Min: 10, Width: titleW},
	}
	narrow := p.Width() < narrowWidth
	if narrow {
		cols = []ui.Column{cols[0], cols[2]} // severity and title move under the row
	}
	section := func(key string, list []engine.RuleResult, withNotes bool) {
		if len(list) == 0 {
			return
		}
		p.Println("")
		p.Println(p.Section(i18n.N(key, len(list))))
		rows := make([][]string, len(list))
		notes := make([]string, len(list))
		for i, res := range list {
			rows[i] = []string{p.Status(string(res.Status), res.Rule.Audit.Type == "manual"), p.Chip(res.Rule.Severity), res.Rule.ID, res.Rule.Title}
			if narrow {
				rows[i] = []string{rows[i][0], res.Rule.ID}
			}
			details := ""
			if withNotes {
				details = res.Details
			}
			if narrow {
				notes[i] = strings.TrimRight(p.Chip(res.Rule.Severity), " ") + " " + p.Bold(res.Rule.Title)
				if details != "" {
					notes[i] += "\n" + p.Dim(strings.Join(strings.Fields(details), " "))
				}
			} else {
				notes[i] = details
			}
		}
		noteCol := 2
		if narrow {
			noteCol = 0
		}
		p.Print(p.Table(cols, rows, notes, ui.TableOpts{Indent: 2, NoteCol: noteCol}))
	}
	section("cli.ui.results.passed", pass, false)
	section("cli.ui.results.skipped", skip, true)
	section("cli.ui.results.failed", fail, true)
	p.Println("")
}

// narrowWidth is the terminal width below which tables stack their long text under the row.
const narrowWidth = 72

// summaryOptions are what the summary card and next-step hints need besides the report.
type summaryOptions struct {
	duration time.Duration
	reports  []string // files written
	remote   bool
	device   bool
	format   string
	flags    string // scan flags to repeat in suggested commands
}

func summaryCard(p *ui.Printer, r *engine.ScanReport, o summaryOptions) string {
	c := countResults(r)
	g := p.Glyphs()
	inner := min(p.Width()-1, ui.SectionWidth) - 4
	label := i18n.T("cli.ui.summary.compliance")
	gw := inner - ui.Width(label) - 10
	if gw > 24 {
		gw = 24
	}
	if gw < 5 {
		gw = 5
	}
	lines := []string{p.Bold(label) + "  " + p.Gauge(r.Score(), gw, fmt.Sprintf("%.1f%%", r.Score())), ""}
	counts := []string{
		p.Good(g.OK) + " " + i18n.N("cli.ui.summary.passed", c.passed),
		p.Bad(g.Fail) + " " + i18n.N("cli.ui.summary.failed", c.failed),
	}
	if c.errs > 0 {
		counts = append(counts, p.Caution(g.Warn)+" "+i18n.N("cli.ui.summary.errors", c.errs))
	}
	counts = append(counts, p.Dim(g.Skip)+" "+i18n.N("cli.ui.summary.skipped", c.skipped),
		p.Accent(g.Manual)+" "+i18n.N("cli.ui.summary.manual", c.manual))
	// Wrap the counts over as many lines as the card needs.
	cur := ""
	for _, part := range counts {
		if cur != "" && ui.Width(cur)+3+ui.Width(part) > inner {
			lines = append(lines, cur)
			cur = ""
		}
		if cur != "" {
			cur += "   "
		}
		cur += part
	}
	lines = append(lines, cur)
	host := r.OS + "/" + r.Arch
	if r.Hostname != "" {
		host = r.Hostname + " " + host
	}
	lines = append(lines, p.Dim(i18n.N("cli.ui.summary.scope", r.TotalRules, "host", host, "duration", shortDuration(o.duration))))

	failed := 0
	bySev := map[string]int{}
	byCat := map[string]int{}
	for _, res := range r.Results {
		if res.Status == engine.StatusFail || res.Status == engine.StatusErr {
			failed++
			bySev[strings.ToLower(res.Rule.Severity)]++
			byCat[category(res.Rule)]++
		}
	}
	if failed == 0 {
		if c.passed > 0 {
			lines = append(lines, "", p.Good(g.OK+" "+i18n.T("cli.ui.summary.allPass")))
		}
		return p.Card(i18n.T("cli.ui.summary.title"), lines)
	}
	lines = append(lines, "", p.Bold(i18n.T("cli.ui.summary.bySeverity")))
	sevLine := ""
	for _, s := range []string{"critical", "high", "medium", "low"} {
		if bySev[s] == 0 {
			continue
		}
		part := strings.TrimRight(p.Chip(s), " ") + " " + p.Bold(fmt.Sprint(bySev[s]))
		if sevLine != "" && ui.Width(sevLine)+2+ui.Width(part) > inner-2 {
			lines = append(lines, "  "+sevLine)
			sevLine = ""
		}
		if sevLine != "" {
			sevLine += "  "
		}
		sevLine += part
	}
	if sevLine != "" {
		lines = append(lines, "  "+sevLine)
	}

	type kv struct {
		k string
		n int
	}
	cats := make([]kv, 0, len(byCat))
	for k, n := range byCat {
		cats = append(cats, kv{k, n})
	}
	sort.Slice(cats, func(i, j int) bool {
		if cats[i].n != cats[j].n {
			return cats[i].n > cats[j].n
		}
		return cats[i].k < cats[j].k
	})
	if len(cats) > 5 {
		cats = cats[:5]
	}
	lines = append(lines, "", p.Bold(i18n.T("cli.ui.summary.topFailing")))
	lw := 0
	for _, c := range cats {
		lw = max(lw, ui.Width(c.k))
	}
	lw = min(lw, inner/3)
	nw := len(fmt.Sprint(cats[0].n))
	bw := min(inner-2-lw-2-nw-2, 30)
	for _, c := range cats {
		lines = append(lines, "  "+ui.Fit(c.k, lw, g.Ellipsis)+"  "+ui.PadLeft(fmt.Sprint(c.n), nw)+"  "+p.Bar(c.n, cats[0].n, bw, ui.Red))
	}
	return p.Card(i18n.T("cli.ui.summary.title"), lines)
}

// nextSteps suggests what to run next.
func nextSteps(p *ui.Printer, r *engine.ScanReport, o summaryOptions) string {
	c := countResults(r)
	var hints []string
	if c.failed+c.errs > 0 && !o.remote && !o.device {
		hints = append(hints, i18n.T("cli.ui.next.applyDryRun", "command", code(strings.TrimSpace("tmaegis apply --dry-run "+o.flags))))
	}
	if o.format == "text" {
		hints = append(hints, i18n.T("cli.ui.next.html", "command", code(strings.TrimSpace("tmaegis scan "+o.flags)+" -f html -o tmaegis-report.html")))
	}
	if c.manual == 0 && !scanIncludeManual && !o.device {
		hints = append(hints, i18n.T("cli.ui.next.manual", "command", code("--include-manual")))
	}
	var b strings.Builder
	if len(o.reports) > 0 {
		b.WriteString("\n" + p.Section(i18n.N("cli.ui.summary.reports", len(o.reports))) + "\n")
		for _, f := range o.reports {
			b.WriteString("  " + p.Accent(p.Glyphs().Arrow) + " " + p.Link(f) + "\n")
		}
	}
	if len(hints) > 0 {
		b.WriteString("\n" + p.Section(i18n.T("cli.ui.next.title")) + "\n")
		for _, h := range hints {
			b.WriteString(p.Hint(h) + "\n")
		}
	}
	return b.String()
}

// finishScan prints the human-readable outcome of a scan. In text format stdout gets the
// results (styled on a terminal, the legacy summary otherwise); the summary card goes to
// whichever of stdout/stderr is a terminal, never into a pipe.
func finishScan(r *engine.ScanReport, o summaryOptions, textOut io.Writer) {
	outP := printerFor(textOut)
	if o.format == "text" {
		if outP.Fancy() {
			writeResultsTable(outP, r)
			outP.Print(summaryCard(outP, r, o))
			outP.Print(nextSteps(outP, r, o))
			return
		}
		r.WriteSummary(textOut)
	}
	if ep := errUI(); ep.Fancy() {
		ep.Println("")
		ep.Print(summaryCard(ep, r, o))
		ep.Print(nextSteps(ep, r, o))
	}
}

// hostHeader separates the hosts of an inventory scan.
func hostHeader(w io.Writer, title string) {
	p := printerFor(w)
	if !p.Fancy() {
		p.Println(title)
		return
	}
	p.Println("")
	p.Println(p.Section(strings.Trim(title, "= ")))
}

// printRemoteSummaryTable is the terminal rendering of the inventory summary.
func printRemoteSummaryTable(p *ui.Printer, results []*remoteResult) {
	p.Println("")
	p.Println(p.Line(i18n.N("cli.scan.ssh.summary.title", len(results))))
	cols := []ui.Column{
		{Title: i18n.T("cli.ui.col.host"), Flex: true, Min: 12},
		{Title: i18n.T("cli.ui.kv.target")},
		{Title: i18n.T("cli.ui.col.result"), Flex: true, Min: 10},
	}
	var rows [][]string
	for _, res := range results {
		if res.err != nil {
			rows = append(rows, []string{res.job.ep.String(), res.job.target, p.Bad(p.Glyphs().Fail + " " + res.err.Error())})
			continue
		}
		r := res.report
		rows = append(rows, []string{res.job.ep.String(), res.job.target,
			p.Gauge(r.Score(), 10, fmt.Sprintf("%.1f%%", r.Score())) + "  " +
				p.Dim(i18n.T("cli.ui.summary.hostCounts", "passed", r.PassedCount, "failed", r.FailedCount, "total", r.TotalRules))})
	}
	p.Print(p.Table(cols, rows, nil, ui.TableOpts{Indent: 2}))
}

// shortDuration formats d for people: 850ms, 4.2s, 3m10s.
func shortDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	case d < time.Minute:
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}
