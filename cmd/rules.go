// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/rules"
	"github.com/tmaegis/tmaegis/internal/ui"
)

var rulesOutput string

var rulesCmd = &cobra.Command{Use: "rules"}

var rulesListCmd = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := rules.LoadFromDir(rulesDir, "all", 2)
		if err != nil {
			return withHint(err, rulesDirHint())
		}
		if p := outUI(); p.Fancy() {
			writeRulesTable(p, all)
			return nil
		}
		for _, r := range all {
			fmt.Printf("%-40s L%d %-8s %-10s %s\n", r.ID, r.Level, r.Severity, r.Target, r.Title)
		}
		fmt.Println(i18n.N("cli.rules.count", len(all)))
		return nil
	},
}

var rulesValidateCmd = &cobra.Command{
	Use: "validate",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := rules.LoadFromDir(rulesDir, "all", 2)
		if err != nil {
			return withHint(err, i18n.T("cli.ui.hint.validate"))
		}
		if p := outUI(); p.Fancy() {
			p.Println(p.Good(p.Glyphs().OK) + " " + i18n.N("cli.rules.valid", len(all), "dir", rulesDir))
			return nil
		}
		fmt.Println(i18n.N("cli.rules.valid", len(all), "dir", rulesDir))
		return nil
	},
}

// rules export feeds the web portal's Rule Explorer (and any other tooling) with one JSON catalogue.
var rulesExportCmd = &cobra.Command{
	Use: "export",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := rules.LoadFromDir(rulesDir, "all", 2)
		if err != nil {
			return err
		}
		out := os.Stdout
		if rulesOutput != "" {
			f, err := os.Create(rulesOutput)
			if err != nil {
				return err
			}
			defer f.Close()
			out = f
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(all)
	},
}

// writeRulesTable is the terminal rendering of `rules list`.
func writeRulesTable(p *ui.Printer, all []rules.Rule) {
	cols := []ui.Column{
		{Title: i18n.T("cli.ui.col.rule"), Flex: true, Min: 14},
		{Title: i18n.T("cli.ui.col.level")},
		{Title: i18n.T("cli.ui.col.severity")},
		{Title: i18n.T("cli.ui.kv.target"), Flex: true, Min: 6},
		{Title: i18n.T("cli.ui.col.title"), Flex: true, Min: 12},
	}
	rows := make([][]string, len(all))
	var notes []string
	narrow := p.Width() < narrowWidth
	if narrow {
		cols = []ui.Column{cols[0], cols[1], cols[3]}
		notes = make([]string, len(all))
	}
	for i, r := range all {
		rows[i] = []string{r.ID, fmt.Sprintf("L%d", r.Level), p.Chip(r.Severity), p.Accent(r.Target), r.Title}
		if narrow {
			rows[i] = []string{r.ID, rows[i][1], rows[i][3]}
			notes[i] = strings.TrimRight(p.Chip(r.Severity), " ") + " " + r.Title
		}
	}
	p.Print(p.Table(cols, rows, notes, ui.TableOpts{Indent: 2}))
	p.Println("")
	p.Println("  " + p.Bold(i18n.N("cli.rules.count", len(all))))
	p.Println(p.Hint(i18n.T("cli.ui.hint.rulesExport", "command", code("tmaegis rules export -o rules.json"))))
}

func init() {
	rulesExportCmd.Flags().StringVarP(&rulesOutput, "output", "o", "", "")
	rulesCmd.AddCommand(rulesListCmd, rulesValidateCmd, rulesExportCmd)
	rootCmd.AddCommand(rulesCmd)
}
