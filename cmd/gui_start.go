// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/tmaegis/tmaegis/internal/config"
	"github.com/tmaegis/tmaegis/internal/gui"
	"github.com/tmaegis/tmaegis/internal/i18n"
)

// StartGUI starts the local GUI server on a random loopback port with the same defaults as
// `tmaegis gui` and returns its URL (it carries the per-run token) and a function that serves
// until ctx is cancelled. The desktop app (cmd/tmaegis-desktop) shows this URL in a native window.
func StartGUI(ctx context.Context, lang string, out io.Writer) (string, func() error, error) {
	c, err := config.Load("config.yaml")
	if err != nil {
		c = config.Default()
	}
	if lang == "" {
		lang = i18n.Detect()
	}
	dir := resolveRulesDir(c.Engine.RulesDir)
	srv, err := gui.New(gui.Options{
		Lang: i18n.Resolve(lang), RulesDir: dir,
		SnapshotDir: c.Engine.SnapshotDir, Concurrency: c.Engine.Concurrency, DeadmanSec: c.Engine.DeadmanTimeout,
		Version: Version,
		AI:      gui.AIInfo{Provider: c.AI.Provider, Model: c.AI.Model, Endpoint: c.AI.Endpoint, KeyEnv: c.AI.APIKeyEnv},
		Out:     out,
	})
	if err != nil {
		return "", nil, err
	}
	l, err := srv.Listen()
	if err != nil {
		return "", nil, fmt.Errorf("cannot start the GUI server: %w", err)
	}
	return srv.URL(), func() error { return srv.Serve(ctx, l) }, nil
}
