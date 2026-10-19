// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil || cfg.Engine.RulesDir != "./rules" || cfg.AI.Provider != "ollama" {
		t.Fatalf("missing file should yield defaults, got %+v %v", cfg, err)
	}

	p := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(p, []byte("ai:\n  provider: claude\nengine:\n  snapshot_dir: /tmp/s\n"), 0o600)
	cfg, err = Load(p)
	if err != nil || cfg.AI.Provider != "claude" || cfg.Engine.SnapshotDir != "/tmp/s" || cfg.Engine.RulesDir != "./rules" {
		t.Fatalf("override failed: %+v %v", cfg, err)
	}

	os.WriteFile(p, []byte("ai: [broken"), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("malformed config must error")
	}
}

func TestExampleConfigParses(t *testing.T) {
	if _, err := Load("../../config.example.yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestUILanguage(t *testing.T) {
	if Default().UI.Language != "" {
		t.Error("default language must be empty (auto-detect)")
	}
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte("ui:\n  language: vi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil || c.UI.Language != "vi" {
		t.Fatalf("got %+v, %v", c.UI, err)
	}
}
