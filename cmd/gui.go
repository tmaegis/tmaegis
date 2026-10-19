// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmaegis/tmaegis/internal/gui"
	"github.com/tmaegis/tmaegis/internal/i18n"
)

var (
	guiPort        int
	guiNoBrowser   bool
	guiLang        string
	guiOpenTimeout time.Duration
	guiIdleTimeout time.Duration
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the local web interface (127.0.0.1 only, protected by a random per-run token)",
	Long: `Starts a local web GUI embedded in this binary. It listens on 127.0.0.1 only, makes no
external requests and requires the per-run token printed below. Applying changes needs
administrator/root privileges. Press Ctrl+C or use the Quit button to stop it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := guiLang
		if lang == "" {
			lang = i18n.Detect()
		}
		srv, err := gui.New(gui.Options{
			Port: guiPort, Lang: i18n.Resolve(lang), RulesDir: rulesDir,
			SnapshotDir: cfg.Engine.SnapshotDir, Concurrency: cfg.Engine.Concurrency, DeadmanSec: cfg.Engine.DeadmanTimeout,
			Version: Version, OpenTimeout: guiOpenTimeout, IdleTimeout: guiIdleTimeout, OSFamily: osFamilyOverride,
			AI:  gui.AIInfo{Provider: firstNonEmpty(aiProvider, cfg.AI.Provider), Model: cfg.AI.Model, Endpoint: cfg.AI.Endpoint, KeyEnv: cfg.AI.APIKeyEnv},
			Out: os.Stderr,
		})
		if err != nil {
			return err
		}
		l, err := srv.Listen()
		if err != nil {
			return fmt.Errorf("cannot start the GUI server: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[*] TMAegis v%s GUI listening on 127.0.0.1 only.\n", Version)
		fmt.Fprintf(os.Stderr, "[*] Open this address in your browser (it contains this run's access token):\n    %s\n", srv.URL())
		fmt.Fprintln(os.Stderr, "[*] Press Ctrl+C to stop.")
		if !guiNoBrowser {
			if err := gui.OpenBrowser(srv.URL()); err != nil {
				fmt.Fprintf(os.Stderr, "[!] Could not open a browser (%v); open the address above manually.\n", err)
			}
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return srv.Serve(ctx, l)
	},
}

func init() {
	guiCmd.Flags().IntVar(&guiPort, "port", 0, "Port to listen on (default: a random free port)")
	guiCmd.Flags().BoolVar(&guiNoBrowser, "no-browser", false, "Do not open a browser; just print the URL")
	guiCmd.Flags().StringVar(&guiLang, "lang", "", "Initial UI language, e.g. vi, ja, zh-Hans (default: detected from the system)")
	guiCmd.Flags().DurationVar(&guiOpenTimeout, "open-timeout", 10*time.Minute, "Exit if no browser has connected within this time (0 disables)")
	guiCmd.Flags().DurationVar(&guiIdleTimeout, "idle-timeout", 0, "Exit after this long without any UI activity or running job (0 disables)")
	rootCmd.AddCommand(guiCmd)
}
