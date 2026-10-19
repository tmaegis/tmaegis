// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmaegis/tmaegis/internal/config"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/ui"
)

// langFlag is the value of the persistent --lang flag ("" = not given, "list" prints the locales).
var langFlag string

// translator is the part of *i18n.Translator the command localisation needs.
type translator interface {
	Has(key string) bool
	T(key string, args ...any) string
}

// resolveLanguage applies the precedence: --lang flag, then TMAEGIS_LANG, then ui.language
// from the config file, then the operating-system locale (i18n.Detect), then en-US.
func resolveLanguage(flag, cfgLang string) string {
	if flag != "" {
		return i18n.Resolve(flag)
	}
	if env := os.Getenv("TMAEGIS_LANG"); env != "" {
		return i18n.Resolve(env)
	}
	if cfgLang != "" {
		return i18n.Resolve(cfgLang)
	}
	return i18n.Detect()
}

// argValue extracts "--name value" / "--name=value" (or the shorthand) from raw arguments, so the
// language is known before cobra renders --help. Parsing stops at "--".
func argValue(args []string, name, short string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		switch {
		case a == "--"+name || (short != "" && a == "-"+short):
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--"+name+"="):
			return strings.TrimPrefix(a, "--"+name+"=")
		case short != "" && strings.HasPrefix(a, "-"+short) && len(a) > 2 && a[2] != '-':
			return strings.TrimPrefix(strings.TrimPrefix(a[2:], "="), " ")
		}
	}
	return ""
}

// languageFromArgs works out the UI language from raw arguments (best effort: a missing or
// malformed config file is ignored here and reported later by PersistentPreRunE).
func languageFromArgs(args []string) (lang string, list bool) {
	flag := argValue(args, "lang", "")
	if flag == "list" {
		return resolveLanguage("", ""), true
	}
	cfgLang := ""
	if flag == "" && os.Getenv("TMAEGIS_LANG") == "" {
		path := argValue(args, "config", "c")
		if path == "" {
			path = "config.yaml"
		}
		if c, err := config.Load(path); err == nil {
			cfgLang = c.UI.Language
		}
	}
	return resolveLanguage(flag, cfgLang), false
}

// PrintLanguages writes the supported locales with their endonyms, marking the active one.
func PrintLanguages(w io.Writer) {
	active := i18n.Lang()
	if p := printerFor(w); p.Fancy() {
		printLanguagesStyled(p, active)
		return
	}
	fmt.Fprintln(w, i18n.T("cli.lang.header"))
	for _, tag := range i18n.Tags {
		mark := " "
		if tag == active {
			mark = "*"
		}
		fmt.Fprintf(w, " %s %-8s %s\n", mark, tag, i18n.Label(tag))
	}
	fmt.Fprintln(w, i18n.T("cli.lang.active", "tag", active, "name", i18n.Label(active)))
}

// printLanguagesStyled is the terminal rendering of the language list.
func printLanguagesStyled(p *ui.Printer, active string) {
	g := p.Glyphs()
	p.Println(p.Section(i18n.T("cli.lang.header")))
	for _, tag := range i18n.Tags {
		mark, tagS, name := p.Dim(g.Skip), ui.PadRight(tag, 8), i18n.Label(tag)
		if !p.Caps().Unicode {
			mark = " "
		}
		if tag == active {
			mark, tagS, name = p.Good(g.Info), p.Good(p.Bold(tagS)), p.Bold(name)
			if !p.Caps().Unicode {
				mark = p.Good("*")
			}
		}
		p.Println("  " + mark + " " + tagS + " " + name)
	}
	p.Println("")
	p.Println(p.Hint(i18n.T("cli.lang.active", "tag", active, "name", i18n.Label(active))))
	p.Println(p.Hint(i18n.T("cli.ui.hint.lang", "command", code("tmaegis --lang vi scan"))))
}

var langCmd = &cobra.Command{
	Use: "lang",
	Run: func(cmd *cobra.Command, args []string) { PrintLanguages(cmd.OutOrStdout()) },
}

// applyLanguage activates lang process-wide and re-localises the command tree.
func applyLanguage(lang string) {
	i18n.SetLang(lang)
	localizeCommands(rootCmd, i18n.New(lang))
}

// flagArgs supplies runtime values some flag descriptions interpolate.
var flagArgs = map[string][]any{}

func cmdKey(c *cobra.Command) string {
	if !c.HasParent() {
		return "root"
	}
	path := strings.TrimPrefix(c.CommandPath(), c.Root().Name()+" ")
	return strings.ReplaceAll(path, " ", ".")
}

// localizeCommands sets Short, Long, and flag descriptions of the whole command tree from the
// catalog (keys cli.cmd.<path>.short|long and cli.flag.<path>.<flag>). Cobra evaluates help
// text lazily, so this can run after the language is known. Entries without a catalog key
// keep their current text.
func localizeCommands(root *cobra.Command, tr translator) {
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		key := cmdKey(c)
		if k := "cli.cmd." + key + ".short"; tr.Has(k) {
			c.Short = tr.T(k)
		}
		if k := "cli.cmd." + key + ".example"; tr.Has(k) {
			c.Example = tr.T(k)
		}
		if k := "cli.cmd." + key + ".long"; tr.Has(k) {
			c.Long = tr.T(k)
			if !c.HasParent() {
				c.Long = Banner() + "\n" + c.Long
			}
		}
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			switch f.Name {
			case "help", "version":
				if k := "cli.flag." + f.Name; tr.Has(k) {
					f.Usage = tr.T(k, "name", c.DisplayName())
				}
			default:
				k := "cli.flag." + key + "." + f.Name
				if tr.Has(k) {
					f.Usage = tr.T(k, flagArgs[key+"."+f.Name]...)
				}
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	localizeGroups(root, tr)
	root.SetUsageTemplate(localizedUsageTemplate(tr))
	root.SetHelpTemplate(helpTemplate)
}
