// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/config"
	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/ui"
)

var (
	Version    = "0.1.0"
	cfgFile    string
	aiProvider string
	// osFamilyOverride replaces the detected os_family (e.g. windows-11); "*" disables the filter.
	osFamilyOverride string
	rulesDir         string
	cfg              = config.Default()
)

var rootCmd = &cobra.Command{
	Use: "tmaegis",
	// Short, Long and flag descriptions come from the i18n catalog (see localizeCommands).
	// Only " (beta)" is appended to the plain version, so scripts that parse the first part keep working.
	Version:      Version + " (beta)",
	SilenceUsage: true,
	// Errors are printed by Execute, styled and with a fix hint.
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// On Windows, double-clicking tmaegis.exe (0 arguments) launches the local GUI directly.
		if runtime.GOOS == "windows" && len(args) == 0 {
			return guiCmd.RunE(guiCmd, args)
		}
		return cmd.Help()
	},
}

func Execute() {
	initConsole()
	rawOpts = optionsFromArgs(os.Args[1:])
	// The language must be known before cobra renders --help, which runs before any PreRun hook.
	lang, list := languageFromArgs(os.Args[1:])
	applyLanguage(lang)
	if list {
		PrintLanguages(os.Stdout)
		return
	}
	if c, err := rootCmd.ExecuteC(); err != nil {
		path := rootCmd.CommandPath()
		if c != nil {
			path = c.CommandPath()
		}
		printError(err, path)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		loaded, err := config.Load(cfgFile)
		if err != nil {
			return withHint(err, i18n.T("cli.ui.hint.config", "path", cfgFile))
		}
		cfg = loaded
		// Language: --lang, TMAEGIS_LANG, ui.language, OS locale.
		applyLanguage(resolveLanguage(langFlag, cfg.UI.Language))
		// Explicit flags win over the config file.
		if !cmd.Flags().Changed("rules-dir") {
			rulesDir = cfg.Engine.RulesDir
		}
		rulesDir = resolveRulesDir(rulesDir)
		if f := cmd.Flags().Lookup("plugin-dir"); f != nil && !f.Changed {
			pluginDir = cfg.Plugins.Directory
		}
		return nil
	}
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", "")
	rootCmd.PersistentFlags().StringVar(&aiProvider, "ai-provider", "", "")
	rootCmd.PersistentFlags().StringVar(&osFamilyOverride, "os-family", "", "")
	rootCmd.PersistentFlags().StringVarP(&rulesDir, "rules-dir", "r", "./rules", "")
	rootCmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "")
	rootCmd.PersistentFlags().BoolVar(&asciiFlag, "ascii", false, "")
	rootCmd.AddCommand(langCmd)
}

// Banner returns the one-line plain banner (name, version, tagline). Terminals get the
// shield logo instead (see PrintBanner and the help template).
func Banner() string {
	return ui.New(io.Discard, ui.Plain).CompactBanner(bannerInfo()) + "\n"
}

// PrintBanner writes the banner to w (stderr for commands whose stdout is machine-readable):
// the shield logo on a terminal, one plain line otherwise.
func PrintBanner(w io.Writer) {
	p := printerFor(w)
	fmt.Fprintln(w, p.Banner(bannerInfo()))
}
