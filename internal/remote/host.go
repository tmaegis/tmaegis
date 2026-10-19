// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package remote

import (
	"bytes"
	"context"
	"strings"

	"github.com/tmaegis/tmaegis/internal/discovery"
	"github.com/tmaegis/tmaegis/internal/engine"
)

// HostExecutor runs audit rules on a remote shell host (Linux, Unix, macOS, or Windows with
// OpenSSH). It implements engine.Remote.
type HostExecutor struct {
	Client *Client
	Info   engine.Target
}

var _ engine.Remote = (*HostExecutor)(nil)

func (h *HostExecutor) Run(ctx context.Context, command, shell string) ([]byte, error) {
	if shell == "" && h.Info.OS == "windows" {
		shell = "powershell"
	}
	return h.Client.Exec(ctx, HostCommand(command, shell))
}

func (h *HostExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if h.Info.OS == "windows" {
		return h.Client.Exec(ctx, HostCommand("Get-Content -Raw -LiteralPath '"+strings.ReplaceAll(path, "'", "''")+"'", "powershell"))
	}
	return h.Client.Exec(ctx, "cat -- "+ShQuote(path))
}

func (h *HostExecutor) Exists(ctx context.Context, path string) bool {
	if h.Info.OS == "windows" {
		out, err := h.Client.Exec(ctx, HostCommand("Test-Path -LiteralPath '"+strings.ReplaceAll(path, "'", "''")+"'", "powershell"))
		return err == nil && strings.TrimSpace(string(out)) == "True"
	}
	_, err := h.Client.Exec(ctx, "test -e "+ShQuote(path))
	return err == nil
}

func (h *HostExecutor) Target() engine.Target { return h.Info }

// Commander runs one command and returns its output; *Client satisfies it via Exec.
type Commander interface {
	Exec(ctx context.Context, command string) ([]byte, error)
}

// windowsProbe matches the one internal/discovery runs locally ("InstallationType|CurrentBuild|DomainRole").
const windowsProbe = `$cv = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'; ` +
	`$role = (Get-CimInstance Win32_ComputerSystem).DomainRole; "$($cv.InstallationType)|$($cv.CurrentBuild)|$role"`

// DetectHost identifies the remote OS and os_family the way discovery.Detect does locally:
// /etc/os-release on Linux, freebsd-version, sw_vers, or the Windows registry. Installed
// software is not probed remotely in this phase.
func DetectHost(ctx context.Context, c Commander) (discovery.Host, engine.Target) {
	var h discovery.Host
	var t engine.Target
	if out, err := c.Exec(ctx, "uname -s; uname -m; uname -n"); err == nil {
		lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r", "")), "\n")
		if len(lines) >= 1 {
			h.OS = strings.ToLower(strings.TrimSpace(lines[0]))
		}
		if len(lines) >= 2 {
			t.Arch = normalizeArch(strings.TrimSpace(lines[1]))
		}
		if len(lines) >= 3 {
			t.Hostname = strings.TrimSpace(lines[2])
		}
	}
	switch h.OS {
	case "linux":
		if out, err := c.Exec(ctx, "cat /etc/os-release 2>/dev/null || cat /usr/lib/os-release"); err == nil {
			h.Distro, h.Version = discovery.ParseOSRelease(bytes.NewReader(out))
			h.Family = discovery.Family(h.Distro, h.Version)
		}
	case "freebsd":
		if out, err := c.Exec(ctx, "freebsd-version -u"); err == nil {
			h.Distro, h.Version = "freebsd", strings.TrimSpace(string(out))
			major, _, _ := strings.Cut(h.Version, ".")
			h.Family = "freebsd-" + major
		}
	case "darwin":
		if out, err := c.Exec(ctx, "sw_vers -productVersion"); err == nil {
			h.Distro, h.Version = "macos", strings.TrimSpace(string(out))
			h.Family = discovery.Family(h.Distro, h.Version)
		}
	case "sunos":
		h.OS = "solaris"
	case "aix":
	case "":
		if out, err := c.Exec(ctx, HostCommand(windowsProbe, "powershell")); err == nil {
			if d, v, f := discovery.ParseWindows(string(out)); f != "" {
				h.OS, h.Distro, h.Version, h.Family = "windows", d, v, f
				t.Arch = "amd64"
			}
		}
		if h.OS == "windows" {
			if out, err := c.Exec(ctx, HostCommand("$env:COMPUTERNAME", "powershell")); err == nil {
				t.Hostname = strings.TrimSpace(string(out))
			}
		}
	}
	t.OS = h.OS
	return h, t
}

func normalizeArch(m string) string {
	switch m {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	}
	return m
}
