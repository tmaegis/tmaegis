// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import "testing"

func TestResolveRulesDir(t *testing.T) {
	has := func(dirs ...string) func(string) bool {
		return func(p string) bool {
			for _, d := range dirs {
				if p == d {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		name, dir, env, exe string
		exists              func(string) bool
		want                string
	}{
		{"explicit path is kept", "/custom", "", "/opt/t", has("/usr/share/tmaegis/rules"), "/custom"},
		{"environment wins over the default", "./rules", "/env/rules", "/opt/t", has("./rules"), "/env/rules"},
		{"working directory first", "./rules", "", "/opt/t", has("./rules", "/usr/share/tmaegis/rules"), "./rules"},
		{"next to the executable", "./rules", "", "/opt/t", has("/opt/t/rules", "/usr/share/tmaegis/rules"), "/opt/t/rules"},
		{"homebrew layout", "./rules", "", "/opt/homebrew/bin", has("/opt/homebrew/share/tmaegis/rules"), "/opt/homebrew/share/tmaegis/rules"},
		{"package location", "./rules", "", "/usr/bin", has("/usr/share/tmaegis/rules"), "/usr/share/tmaegis/rules"},
		{"container location", "./rules", "", "/usr/local/bin", has("/etc/tmaegis/rules"), "/etc/tmaegis/rules"},
		{"nothing found keeps the default", "./rules", "", "/usr/bin", has(), "./rules"},
	}
	for _, c := range cases {
		if got := resolveRulesDirIn(c.dir, c.env, c.exe, c.exists); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
