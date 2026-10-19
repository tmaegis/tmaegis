// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bytes"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/ui"
)

// Command groups shown by `tmaegis --help` (titles come from cli.help.group.<id>).
var helpGroups = []struct {
	id   string
	cmds []*cobra.Command
}{
	{"harden", []*cobra.Command{scanCmd, applyCmd, rollbackCmd}},
	{"inspect", []*cobra.Command{discoverCmd, rulesCmd, langCmd}},
	{"extend", []*cobra.Command{guiCmd, pluginCmd}},
}

func init() {
	for _, g := range helpGroups {
		rootCmd.AddGroup(&cobra.Group{ID: g.id, Title: g.id})
		for _, c := range g.cmds {
			c.GroupID = g.id
		}
	}
	cobra.AddTemplateFuncs(template.FuncMap{
		"tmHeading": helpHeading,
		"tmIntro":   helpIntro,
		"tmCmds":    helpCommands,
		"tmFlags":   helpFlags,
		"tmExample": helpExample,
		"tmFooter":  helpFooter,
	})
}

// localizeGroups sets the group titles for the active language.
func localizeGroups(root *cobra.Command, tr translator) {
	for _, g := range root.Groups() {
		if k := "cli.help.group." + g.ID; tr.Has(k) {
			g.Title = tr.T(k)
		}
	}
}

// groupOrder keeps the curated order of helpGroups (scan before apply before rollback).
func groupOrder(c *cobra.Command) int {
	for _, g := range helpGroups {
		for i, gc := range g.cmds {
			if gc == c {
				return i
			}
		}
	}
	return 99
}

// helpTarget is the writer a help page is going to. Cobra renders the usage part into a
// buffer (UsageString), so the help template records the real destination first.
var helpTarget io.Writer

func helpPrinter(c *cobra.Command) *ui.Printer {
	w := c.OutOrStdout()
	if _, buffered := w.(*bytes.Buffer); buffered && helpTarget != nil {
		w = helpTarget
	}
	return printerFor(w)
}

func helpHeading(c *cobra.Command, title string) string {
	p := helpPrinter(c)
	if !p.Fancy() {
		return title
	}
	return p.Paint(strings.TrimSuffix(title, ":"), ui.Style{FG: &ui.Cyan, Bold: true})
}

// helpIntro is the banner and description at the top of a help page.
func helpIntro(c *cobra.Command) string {
	helpTarget = c.OutOrStdout()
	p := helpPrinter(c)
	text := c.Long
	if text == "" {
		text = c.Short
	}
	if !p.Fancy() {
		if text == "" {
			return ""
		}
		return strings.TrimRight(text, " \n") + "\n\n"
	}
	var b strings.Builder
	if !c.HasParent() {
		b.WriteString(p.Banner(bannerInfo()) + "\n")
		text = strings.TrimPrefix(text, Banner())
		text = strings.TrimLeft(text, "\n")
	} else {
		b.WriteString(p.CompactBanner(bannerInfo()) + "\n\n")
	}
	if text != "" {
		text = strings.TrimRight(text, " \n")
		for _, l := range strings.Split(text, "\n") {
			if ui.Width(l) > p.Width()-3 {
				text = ui.JoinLines(text) // re-flow hard-wrapped text for this terminal
				break
			}
		}
		for _, l := range ui.Wrap(text, p.Width()-3) {
			b.WriteString(" " + l + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// helpCommands lists subcommands, under their group titles when the command has groups.
func helpCommands(c *cobra.Command) string {
	p := helpPrinter(c)
	var avail []*cobra.Command
	nameW := 0
	for _, s := range c.Commands() {
		if s.IsAvailableCommand() || s.Name() == "help" {
			avail = append(avail, s)
			nameW = max(nameW, ui.Width(s.Name()))
		}
	}
	row := func(indent string, s *cobra.Command) string {
		name := ui.PadRight(s.Name(), nameW)
		if p.Fancy() {
			name = p.Paint(name, ui.Style{FG: &ui.Emerald, Bold: true})
		}
		descW := p.Width() - 1 - ui.Width(indent) - nameW - 3
		short := s.Short
		if p.Fancy() && descW > 10 {
			short = ui.Truncate(short, descW, p.Glyphs().Ellipsis)
		}
		return indent + name + "   " + short + "\n"
	}
	var b strings.Builder
	groups := c.Groups()
	if len(groups) == 0 {
		for _, s := range avail {
			b.WriteString(row("  ", s))
		}
		return strings.TrimRight(b.String(), "\n")
	}
	for _, g := range groups {
		var in []*cobra.Command
		for _, s := range avail {
			if s.GroupID == g.ID {
				in = append(in, s)
			}
		}
		sort.SliceStable(in, func(i, j int) bool { return groupOrder(in[i]) < groupOrder(in[j]) })
		if len(in) == 0 {
			continue
		}
		title := g.Title
		if p.Fancy() {
			title = p.Bold(strings.TrimSuffix(title, ":"))
		}
		b.WriteString("  " + title + "\n")
		for _, s := range in {
			b.WriteString(row("    ", s))
		}
	}
	var rest []*cobra.Command
	for _, s := range avail {
		if s.GroupID == "" {
			rest = append(rest, s)
		}
	}
	if len(rest) > 0 {
		title := i18n.T("cli.help.additionalCommands")
		if p.Fancy() {
			title = p.Bold(strings.TrimSuffix(title, ":"))
		}
		b.WriteString("  " + title + "\n")
		for _, s := range rest {
			b.WriteString(row("    ", s))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

var flagLine = regexp.MustCompile(`^(\s+)((?:-\w, )?--[\w-]+)( \w+)?`)

// helpFlags renders a flag set with aligned, wrapped descriptions and coloured names.
func helpFlags(c *cobra.Command, fs *pflag.FlagSet) string {
	p := helpPrinter(c)
	if !p.Fancy() {
		return strings.TrimRight(fs.FlagUsages(), " \n")
	}
	lines := strings.Split(strings.TrimRight(fs.FlagUsagesWrapped(p.Width()-1), " \n"), "\n")
	for i, l := range lines {
		lines[i] = flagLine.ReplaceAllStringFunc(strings.TrimRight(l, " "), func(m string) string {
			sub := flagLine.FindStringSubmatch(m)
			return sub[1] + p.Paint(sub[2], ui.Style{FG: &ui.Emerald}) + p.Dim(sub[3])
		})
	}
	return strings.Join(lines, "\n")
}

// helpExample indents examples; on a terminal comments are dimmed and commands highlighted.
func helpExample(c *cobra.Command) string {
	p := helpPrinter(c)
	lines := strings.Split(strings.TrimRight(c.Example, " \n"), "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case !p.Fancy():
			if t != "" {
				lines[i] = "  " + l
			}
		case strings.HasPrefix(t, "#"):
			lines[i] = "  " + p.Dim(t)
		case t != "":
			lines[i] = "  " + p.Accent(p.Glyphs().Pointer) + " " + p.Paint(t, ui.Style{Bold: true})
		}
	}
	return strings.Join(lines, "\n")
}

// helpFooter is the "more information" line and the --lang hint. more contains a
// {command} placeholder for the command path.
func helpFooter(c *cobra.Command, more string) string {
	p := helpPrinter(c)
	helpTarget = nil
	more = strings.ReplaceAll(more, "{command}", c.CommandPath())
	hint := i18n.N("cli.help.langHint", len(i18n.Tags), "flag", code("--lang"), "list", code("--lang list"))
	out := ""
	if !p.Fancy() {
		if c.HasAvailableSubCommands() {
			out = more + "\n"
		}
		return out + strings.ReplaceAll(hint, "`", "") + "\n"
	}
	if c.HasAvailableSubCommands() {
		out = p.Hint(more) + "\n"
	}
	return out + p.Hint(hint) + "\n"
}

// localizedUsageTemplate is cobra's usage layout with translated headings, grouped
// commands, wrapped flags and an examples section; styling is applied only on terminals.
func localizedUsageTemplate(tr translator) string {
	h := func(key, en string) string {
		s := en
		if tr.Has(key) {
			s = tr.T(key)
		}
		return `{{tmHeading . ` + strconv.Quote(s) + `}}`
	}
	more := `Use "{command} [command] --help" for more information about a command.`
	if tr.Has("cli.help.moreInfo") {
		more = tr.T("cli.help.moreInfo")
	}
	return h("cli.help.usage", "Usage:") + `{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + h("cli.help.aliases", "Aliases:") + `
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}

` + h("cli.help.availableCommands", "Available Commands:") + `
{{tmCmds .}}{{end}}{{if .HasAvailableLocalFlags}}

` + h("cli.help.flags", "Flags:") + `
{{tmFlags . .LocalFlags}}{{end}}{{if .HasAvailableInheritedFlags}}

` + h("cli.help.globalFlags", "Global Flags:") + `
{{tmFlags . .InheritedFlags}}{{end}}{{if .HasExample}}

` + h("cli.help.examples", "Examples:") + `
{{tmExample .}}{{end}}{{if .HasHelpSubCommands}}

` + h("cli.help.additionalTopics", "Additional help topics:") + `{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}

{{tmFooter . ` + strconv.Quote(more) + `}}`
}

// helpTemplate puts the banner and description above the usage.
const helpTemplate = `{{tmIntro .}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
