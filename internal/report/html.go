// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

//go:embed assets/report.html.tmpl assets/report.css assets/report.js assets/logo-mark.svg assets/fonts/*.woff2
var htmlAssets embed.FS

type htmlRule struct {
	N                                                    int
	ID, Title, Target, Severity, SevClass, Status        string
	StatusClass, StatusLabel, Icon                       string
	Details, Description, Reference, Remediation, Search string
	Bench, BenchName                                     string
	Level                                                int
	Guidance                                             bool
	Plan, Risk                                           string // action-plan group and risk hint (failed rules only)
	Instructions                                         string // manual-review instructions
	rank                                                 int
}

type htmlBar struct {
	Class, Key string
	Count, Pct int
}

type htmlBench struct {
	Key, Name, Ref            string
	Pass, Fail, Skip, Total   int
	Score, ScoreText, Class   string
	PassPct, FailPct, SkipPct int
	HasScore                  bool
	scoreN                    float64
	refCount                  map[string]int
}

type htmlHost struct {
	Name, OS, Arch, Targets, When, WhenISO, Duration, RulesDir string
}

var sevRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

func sevClass(s string) string {
	s = strings.ToLower(s)
	if _, ok := sevRank[s]; ok {
		return s
	}
	return "other"
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

func remediationText(r engine.RuleResult) (text string, guidance bool) {
	rem := r.Rule.Remediation
	switch rem.Type {
	case "iac_patch":
		return clip(rem.Command, 1500), true
	case "file_line":
		return clip(fmt.Sprintf("%s\n%s", rem.FilePath, rem.ReplaceLine), 1500), false
	case "command", "sql_exec":
		return clip(rem.Command, 1500), false
	}
	return "", false
}

// serviceAffecting spots commands that restart services, change accounts, firewalls, kernel
// settings or remove data; those belong in "Needs planning", not in "Quick wins".
var serviceAffecting = regexp.MustCompile(`(?i)(systemctl|\bservice\s|restart|reload|reboot|shutdown|\bhalt\b|iptables|ip6tables|\bnft\b|\bufw\b|firewall-cmd|usermod|userdel|useradd|\bpasswd\b|\bchage\b|\bmount\b|umount|sysctl|modprobe|rmmod|update-grub|grub|setenforce|selinux|apparmor|\bkill|rm\s+-[a-z]*r|mkfs|fdisk|parted|crontab|dpkg-reconfigure|(apt|apt-get|yum|dnf|zypper)\s+(remove|purge|erase)|Set-Service|Stop-Service|Restart-Service|Disable-|netsh|reg\s+(add|delete)|\bsc\s+(config|stop)|sshd)`)

// classify puts a failed rule into an action-plan group and gives it a risk hint.
func classify(r engine.RuleResult, rem string) (plan, risk string) {
	if r.Status == engine.StatusErr {
		return "manual", "unknown"
	}
	spec := r.Rule.Remediation
	switch spec.Type {
	case "iac_patch":
		return "planned", "guidance"
	case "sql_exec":
		return "planned", "database"
	case "command", "file_line":
		if strings.TrimSpace(rem) == "" {
			return "manual", "unknown"
		}
		if serviceAffecting.MatchString(spec.Command + " " + spec.ReplaceLine) {
			return "planned", "service"
		}
		return "quick", "low"
	}
	return "manual", "unknown"
}

// benchKey derives the benchmark/product a rule belongs to from its ID ("TMA-DOCKER-1.2.3" ->
// "DOCKER"), falling back to the target.
func benchKey(id, target string) string {
	parts := strings.Split(id, "-")
	var keep []string
	for i, p := range parts {
		if i == 0 && strings.EqualFold(p, "TMA") {
			continue
		}
		if p == "" || p[0] >= '0' && p[0] <= '9' || strings.Contains(p, ".") {
			break
		}
		keep = append(keep, p)
	}
	if len(keep) > 0 {
		key := strings.ToUpper(strings.Join(keep, "-"))
		// "TMA-DOCKER-NO-PRIVILEGED" is a Docker rule, not its own benchmark
		if t := strings.ToUpper(target); t != "" && strings.HasPrefix(key, t+"-") {
			return t
		}
		return key
	}
	if target != "" {
		return strings.ToUpper(target)
	}
	return ""
}

// benchRef trims a rule reference to the benchmark name: "CIS Docker Benchmark v1.9.0 - 1.1.1 x" -> "CIS Docker Benchmark v1.9.0".
func benchRef(ref string) string {
	if i := strings.Index(ref, " - "); i > 0 {
		ref = ref[:i]
	}
	return clip(strings.TrimSpace(ref), 80)
}

func asset(name string) string {
	b, _ := htmlAssets.ReadFile("assets/" + name)
	return string(b)
}

// fontFace embeds a woff2 file so the report works offline and on air-gapped machines.
func fontFace(family string, weight int, file string) string {
	b, err := htmlAssets.ReadFile("assets/fonts/" + file)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("@font-face{font-family:%q;font-weight:%d;font-style:normal;font-display:swap;src:url(data:font/woff2;base64,%s) format(\"woff2\")}\n",
		family, weight, base64.StdEncoding.EncodeToString(b))
}

var circumference = 2 * math.Pi * 52

// htmlTmpl is parsed once with stub helpers; each write clones it and binds its own translator.
var htmlTmpl = func() *template.Template {
	stub := (&ctx{}).funcs()
	return template.Must(template.New("report.html.tmpl").Funcs(stub).Parse(asset("report.html.tmpl")))
}()

// writeHTML emits one self-contained file: fonts, logo, CSS, JS and the i18n dictionary are inline,
// a strict Content-Security-Policy blocks everything else, and html/template escapes all rule text.
func writeHTML(w io.Writer, rep *engine.ScanReport, c *ctx) error {
	tr := c.tr
	var results []htmlRule
	var pass, fail, skip, manual, na int
	sev := map[string]int{}
	targets := map[string]bool{}
	benches := map[string]*htmlBench{}
	var benchOrder []string
	levels := map[int]bool{}
	for i, r := range rep.Results {
		st := strings.ToUpper(string(r.Status))
		hr := htmlRule{
			N: i + 1, ID: r.Rule.ID, Title: r.Rule.Title, Target: r.Rule.Target, Level: r.Rule.Level,
			Severity: tr.T("report.sev." + sevClass(r.Rule.Severity)), SevClass: sevClass(r.Rule.Severity),
			Status: st, Details: clip(r.Details, 4000), Description: clip(r.Rule.Description, 1200), Reference: r.Rule.Reference,
		}
		hr.rank = 4
		if rk, ok := sevRank[hr.SevClass]; ok {
			hr.rank = rk
		}
		bk := benchKey(r.Rule.ID, r.Rule.Target)
		if bk == "" {
			bk = "-"
		}
		hr.Bench = bk
		hr.BenchName = bk
		if bk == "-" {
			hr.BenchName = tr.T("report.bench.other")
		}
		b := benches[bk]
		if b == nil {
			b = &htmlBench{Key: bk, Name: hr.BenchName, refCount: map[string]int{}}
			benches[bk] = b
			benchOrder = append(benchOrder, bk)
		}
		b.Total++
		if ref := benchRef(r.Rule.Reference); ref != "" {
			b.refCount[ref]++
		}
		levels[r.Rule.Level] = true
		switch st {
		case "PASS":
			pass++
			b.Pass++
			hr.StatusClass, hr.StatusLabel, hr.Icon = "pass", tr.T("report.status.pass"), "pass"
		case "SKIP":
			skip++
			b.Skip++
			hr.StatusClass, hr.StatusLabel, hr.Icon = "skip", tr.T("report.status.skip"), "skip"
			if r.Rule.Audit.Type == "manual" {
				manual++
				hr.Plan = "skipmanual"
				hr.Instructions = clip(strings.TrimSpace(r.Rule.Audit.Command), 3000)
				if hr.Instructions == "" {
					hr.Instructions = clip(strings.TrimPrefix(r.Details, "manual review: "), 3000)
				}
			} else {
				na++
				hr.Plan = "skipna"
			}
		case "ERROR":
			fail++
			b.Fail++
			sev[hr.SevClass]++
			hr.StatusClass, hr.StatusLabel, hr.Icon = "error", tr.T("report.status.error"), "fail"
		default:
			fail++
			b.Fail++
			sev[hr.SevClass]++
			hr.Status = "FAIL"
			hr.StatusClass, hr.StatusLabel, hr.Icon = "fail", tr.T("report.status.fail"), "fail"
		}
		hr.Remediation, hr.Guidance = remediationText(r)
		if hr.StatusClass == "fail" || hr.StatusClass == "error" {
			hr.Plan, hr.Risk = classify(r, hr.Remediation)
		}
		if hr.Target != "" {
			targets[hr.Target] = true
		}
		hr.Search = strings.ToLower(hr.ID + " " + hr.Title + " " + hr.Target + " " + hr.Bench + " " + clip(hr.Details, 300))
		results = append(results, hr)
	}

	var top []htmlRule
	groups := map[string][]htmlRule{}
	for _, r := range results {
		if r.StatusClass == "fail" || r.StatusClass == "error" {
			top = append(top, r)
			groups[r.Plan] = append(groups[r.Plan], r)
		}
	}
	bySev := func(s []htmlRule) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].rank < s[j].rank })
	}
	bySev(top)
	for k := range groups {
		bySev(groups[k])
	}
	if len(top) > 6 {
		top = top[:6]
	}
	var skipNA, skipManual []htmlRule
	for _, r := range results {
		switch r.Plan {
		case "skipna":
			skipNA = append(skipNA, r)
		case "skipmanual":
			skipManual = append(skipManual, r)
		}
	}
	bySev(skipManual)

	var bars []htmlBar
	for _, k := range []string{"critical", "high", "medium", "low", "other"} {
		if n := sev[k]; n > 0 || k != "other" {
			pct := 0
			if fail > 0 {
				pct = int(math.Round(float64(n) * 100 / float64(fail)))
			}
			bars = append(bars, htmlBar{Class: k, Key: "report.sev." + k, Count: n, Pct: pct})
		}
	}

	var blist []*htmlBench
	for _, k := range benchOrder {
		b := benches[k]
		best, bn := "", 0
		for ref, n := range b.refCount {
			if n > bn || n == bn && ref < best {
				best, bn = ref, n
			}
		}
		b.Ref = best
		if ev := b.Pass + b.Fail; ev > 0 {
			b.HasScore = true
			b.scoreN = float64(b.Pass) / float64(ev) * 100
			b.Score = fmt.Sprintf("%.1f", b.scoreN)
			b.ScoreText = b.Score + "%"
		}
		b.Class = "good"
		switch {
		case !b.HasScore:
			b.Class = "none"
		case b.scoreN < 70:
			b.Class = "poor"
		case b.scoreN < 90:
			b.Class = "fair"
		}
		if b.Total > 0 {
			b.PassPct = int(math.Round(float64(b.Pass) * 100 / float64(b.Total)))
			b.FailPct = int(math.Round(float64(b.Fail) * 100 / float64(b.Total)))
			b.SkipPct = 100 - b.PassPct - b.FailPct
			if b.SkipPct < 0 {
				b.SkipPct = 0
			}
		}
		blist = append(blist, b)
	}
	sort.SliceStable(blist, func(i, j int) bool {
		if blist[i].Fail != blist[j].Fail {
			return blist[i].Fail > blist[j].Fail
		}
		return blist[i].Key < blist[j].Key
	})

	tl := make([]string, 0, len(targets))
	for k := range targets {
		tl = append(tl, k)
	}
	sort.Strings(tl)
	var lv []string
	for l := range levels {
		lv = append(lv, fmt.Sprintf("L%d", l))
	}
	sort.Strings(lv)

	sc := score(rep)
	verdict := struct{ Class, Key string }{"good", "report.verdict.good"}
	switch {
	case sc < 70:
		verdict.Class, verdict.Key = "poor", "report.verdict.poor"
	case sc < 90:
		verdict.Class, verdict.Key = "fair", "report.verdict.fair"
	}
	evaluated := pass + fail
	urgent := sev["critical"] + sev["high"]
	leadKey, leadN := "report.lead.none", 0
	if evaluated > 0 {
		leadKey = "report.lead.allpass"
		if fail > 0 {
			leadKey, leadN = "report.lead.some", fail
		}
	}
	offset := circumference * (1 - sc/100)

	when := rep.Timestamp
	if when.IsZero() {
		when = time.Now()
	}
	host := htmlHost{
		Name: rep.Hostname, OS: rep.OS, Arch: rep.Arch, Targets: strings.Join(tl, ", "),
		When: when.Local().Format("2006-01-02 15:04 MST"), WhenISO: when.UTC().Format(time.RFC3339),
		RulesDir: c.info.RulesDir,
	}
	if c.info.Duration > 0 {
		host.Duration = c.info.Duration.Round(time.Millisecond).String()
	}
	pageHost := rep.Hostname
	if pageHost == "" {
		pageHost = "-"
	}
	// a stable id for this report so the browser can remember ticked action-plan items per scan
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%d", rep.Hostname, host.WhenISO, len(results))
	for _, r := range results {
		h.Write([]byte(r.ID))
	}
	reportID := hex.EncodeToString(h.Sum(nil))[:16]

	version := c.info.Version
	if version == "" {
		version = "dev"
	}

	css := fontFace("Geist", 400, "Geist-Regular.woff2") + fontFace("Geist", 600, "Geist-SemiBold.woff2") +
		fontFace("Geist Mono", 400, "GeistMono-Regular.woff2") + asset("report.css")
	js := asset("report.js")
	sum := sha256.Sum256([]byte(js))
	csp := fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'sha256-%s'; font-src data:; img-src data:">`,
		base64.StdEncoding.EncodeToString(sum[:]))

	t, err := htmlTmpl.Clone()
	if err != nil {
		return err
	}
	t = t.Funcs(c.funcs())
	// the fragments below come from the binary itself; only rule text (escaped by html/template)
	// is attacker-influenced, so they are safe to mark as HTML/JS.
	return t.Execute(w, map[string]interface{}{
		"Lang": c.tag, "Host": host, "PageHost": pageHost, "ReportID": reportID, "Version": version,
		"Total": len(results), "Passed": pass, "Failed": fail, "Skipped": skip, "Evaluated": evaluated, "Urgent": urgent,
		"Manual": manual, "NA": na,
		"ScoreText": fmt.Sprintf("%.1f", sc), "ScoreNum": fmt.Sprintf("%.1f", sc), "Verdict": verdict, "LeadKey": leadKey, "LeadN": leadN,
		"Circ": fmt.Sprintf("%.2f", circumference), "Offset": fmt.Sprintf("%.2f", offset),
		"Bars": bars, "Top": top, "Targets": tl, "Results": results,
		"Quick": groups["quick"], "Planned": groups["planned"], "Manuals": groups["manual"],
		"SkipNA": skipNA, "SkipManual": skipManual, "Benches": blist,
		"Levels": strings.Join(lv, ", "), "Options": c.info.Options, "LangNames": langOptions(),
		"HasManual": manual > 0,
		"Dict":      dictionary(),
		"CSP":       template.HTML(csp),
		"Style":     template.HTML("<style>" + css + "</style>"),
		"Script":    template.HTML("<script>" + js + "</script>"),
		"Logo":      template.HTML(asset("logo-mark.svg")),
	})
}
