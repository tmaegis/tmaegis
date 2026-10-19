// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"os"
	"path/filepath"
)

// defaultRulesDir is the value of --rules-dir when nothing else is configured.
const defaultRulesDir = "./rules"

// resolveRulesDir turns the default "./rules" into the directory that actually holds the rules
// when TMAegis was installed from a package, a container image or an archive that was not
// unpacked into the working directory. An explicit path (flag, config file) is never changed.
// Order: TMAEGIS_RULES_DIR, ./rules, <executable dir>/rules, <executable dir>/../share/tmaegis/rules, /usr/share/tmaegis/rules,
// /usr/local/share/tmaegis/rules, /etc/tmaegis/rules.
func resolveRulesDir(dir string) string {
	return resolveRulesDirIn(dir, os.Getenv("TMAEGIS_RULES_DIR"), executableDir(), isDir)
}

func resolveRulesDirIn(dir, envDir, exeDir string, exists func(string) bool) string {
	if dir != defaultRulesDir {
		return dir
	}
	if envDir != "" {
		return envDir
	}
	if exists(dir) {
		return dir
	}
	candidates := []string{"/usr/share/tmaegis/rules", "/usr/local/share/tmaegis/rules", "/etc/tmaegis/rules"}
	if exeDir != "" {
		// <exeDir>/rules for unpacked archives, <exeDir>/../share/tmaegis/rules for Homebrew-style layouts.
		candidates = append([]string{filepath.ToSlash(filepath.Join(exeDir, "rules")), filepath.ToSlash(filepath.Clean(filepath.Join(exeDir, "..", "share", "tmaegis", "rules")))}, candidates...)
	}
	for _, c := range candidates {
		if exists(c) {
			return c
		}
	}
	return dir
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
