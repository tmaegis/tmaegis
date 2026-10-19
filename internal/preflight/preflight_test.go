// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package preflight

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCollectLinux(t *testing.T) {
	run := func(_ context.Context, name string, _ ...string) (string, error) {
		switch name {
		case "ps":
			return "nginx\nsshd\nnginx\n\n", nil
		case "ss":
			return "tcp LISTEN 0 511 0.0.0.0:80 0.0.0.0:*\n", nil
		}
		return "", errors.New("unexpected")
	}
	out := collect(context.Background(), run, "linux")
	if !strings.Contains(out, "nginx, sshd") || !strings.Contains(out, "0.0.0.0:80") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCollectSkipsMissingToolsAndTruncates(t *testing.T) {
	none := func(context.Context, string, ...string) (string, error) { return "", errors.New("missing") }
	if out := collect(context.Background(), none, "linux"); out != "" {
		t.Fatalf("want empty, got %q", out)
	}
	big := func(context.Context, string, ...string) (string, error) { return strings.Repeat("x", 10000), nil }
	out := collect(context.Background(), big, "linux")
	if len(out) > maxBytes+30 || !strings.HasSuffix(out, "[truncated]") {
		t.Fatalf("not truncated: %d", len(out))
	}
}

func TestCollectWindowsUsesPowerShell(t *testing.T) {
	var used []string
	run := func(_ context.Context, name string, _ ...string) (string, error) {
		used = append(used, name)
		return "svchost\n", nil
	}
	collect(context.Background(), run, "windows")
	if len(used) != 2 || used[0] != "powershell" {
		t.Fatalf("used %v", used)
	}
}
