// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build desktop

// Command tmaegis-desktop shows the TMAegis GUI in a native window (the system web view:
// WebView2 on Windows, WKWebView on macOS, WebKitGTK on Linux). The GUI server is the same one
// `tmaegis gui` runs: loopback only and protected by a random per-run token. Built with CGO, see
// .github/workflows/desktop.yml. When no web view can be created it opens the default browser.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	webview "github.com/webview/webview_go"

	"github.com/tmaegis/tmaegis/cmd"
	"github.com/tmaegis/tmaegis/internal/gui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	url, serve, err := cmd.StartGUI(ctx, "", os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "TMAegis Desktop:", err)
		os.Exit(1)
	}
	done := make(chan error, 1)
	go func() { done <- serve() }()

	w := webview.New(false)
	if w == nil {
		fmt.Fprintln(os.Stderr, "[!] No native web view is available; opening your browser instead.")
		if err := gui.OpenBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Could not open a browser (%v). Open this address: %s\n", err, url)
		}
		<-done
		return
	}
	defer w.Destroy()
	w.SetTitle("TMAegis")
	w.SetSize(1280, 820, webview.HintNone)
	w.SetSize(960, 640, webview.HintMin)
	w.Navigate(url)
	w.Run()
	stop()
}
