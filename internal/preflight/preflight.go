// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package preflight snapshots what is running on the host (processes, listening ports)
// so the AI can warn when a hardening rule would disrupt a live service.
package preflight

import (
	"context"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maxBytes = 4096

// Runner executes a command and returns its output; swappable for tests.
type Runner func(ctx context.Context, name string, args ...string) (string, error)

func execRunner(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return string(out), err
}

// Collect gathers a bounded, human-readable summary. Missing tools are skipped silently:
// preflight is advisory and must never block a scan.
func Collect(ctx context.Context) string { return collect(ctx, execRunner, runtime.GOOS) }

func collect(ctx context.Context, run Runner, goos string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var b strings.Builder
	if goos == "windows" {
		if out, err := run(ctx, "powershell", "-NoProfile", "-Command", "Get-Process | Select-Object -ExpandProperty ProcessName -Unique"); err == nil {
			b.WriteString("Running processes:\n" + uniqueLines(out) + "\n")
		}
		if out, err := run(ctx, "powershell", "-NoProfile", "-Command", "Get-NetTCPConnection -State Listen | Select-Object -ExpandProperty LocalPort -Unique"); err == nil {
			b.WriteString("Listening TCP ports:\n" + uniqueLines(out) + "\n")
		}
	} else {
		if out, err := run(ctx, "ps", "-eo", "comm="); err == nil {
			b.WriteString("Running processes:\n" + uniqueLines(out) + "\n")
		}
		if out, err := run(ctx, "ss", "-tulnH"); err == nil {
			b.WriteString("Listening sockets (proto recv-q send-q local peer):\n" + strings.TrimSpace(out) + "\n")
		}
	}
	return truncate(b.String(), maxBytes)
}

func uniqueLines(s string) string {
	set := map[string]bool{}
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			set[l] = true
		}
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...[truncated]"
}
