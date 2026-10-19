// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package discovery

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	id, ver := ParseOSRelease(strings.NewReader("NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"))
	if id != "ubuntu" || ver != "22.04" {
		t.Fatalf("got %q %q", id, ver)
	}
	if Family(id, ver) != "ubuntu-22.04" {
		t.Fatal("ubuntu family")
	}
	if got := Family("rocky", "9.3"); got != "rocky-9" {
		t.Fatalf("rocky family = %s", got)
	}
	if Family("", "") != "" {
		t.Fatal("empty distro")
	}
}

func TestDetectSoftware(t *testing.T) {
	have := map[string]bool{"nginx": true, "mariadb": true, "kubectl": true}
	look := func(b string) (string, error) {
		if have[b] {
			return "/usr/bin/" + b, nil
		}
		return "", errors.New("nope")
	}
	got := strings.Join(detectSoftwareWith(look, func(string) bool { return false }), ",")
	if got != "nginx,kubernetes,mariadb" {
		t.Fatalf("got %s", got)
	}
	t.Setenv("ProgramFiles", `C:\Program Files`)
	exists := func(p string) bool { return strings.Contains(filepath.ToSlash(p), "Mozilla Firefox/firefox.exe") }
	if got := strings.Join(detectSoftwareWith(look, exists), ","); got != "nginx,kubernetes,mariadb,firefox" {
		t.Fatalf("path probes: got %s", got)
	}
	if (Host{OS: "darwin"}).Targets()[0] != "macos" {
		t.Fatal("macOS rules use target macos")
	}
	if got := strings.Join((Host{OS: "linux", Distro: "cos"}).Targets(), ","); got != "linux,cos" {
		t.Fatalf("cos targets: got %s", got)
	}
	h := Host{OS: "linux", Software: []string{"nginx"}}
	if len(h.Targets()) != 2 || h.Targets()[0] != "linux" {
		t.Fatal("targets")
	}
}

func TestParseWindows(t *testing.T) {
	cases := []struct{ in, distro, version, family string }{
		{"Server|20348|3\r\n", "windows-server", "2022", "windows-server-2022-ms"},
		{"Server Core|17763|5", "windows-server", "2019", "windows-server-2019-dc"},
		{"Server|26100|4", "windows-server", "2025", "windows-server-2025-dc"},
		{"Server|14393|2", "windows-server", "2016", "windows-server-2016-ms"},
		{"Client|26100|1", "windows-client", "26100", "windows-11"}, // Windows 11 24H2 shares build 26100
		{"Client|26200|1", "windows-client", "26200", "windows-11"},
		{"Client|19045|1", "windows-client", "19045", "windows-10"},
		{"Server|99999|3", "windows-server", "99999", "windows-server-99999"},
		{"garbage", "", "", ""},
	}
	for _, c := range cases {
		d, v, f := ParseWindows(c.in)
		if d != c.distro || v != c.version || f != c.family {
			t.Errorf("%q -> %q %q %q", c.in, d, v, f)
		}
	}
}
