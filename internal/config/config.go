// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package config loads the optional YAML configuration file (see config.example.yaml).
package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	AI      AIConfig      `yaml:"ai"`
	Engine  EngineConfig  `yaml:"engine"`
	Plugins PluginsConfig `yaml:"plugins"`
	UI      UIConfig      `yaml:"ui"`
}

// UIConfig holds presentation settings.
type UIConfig struct {
	// Language is a BCP 47 tag such as "vi" or "zh-Hans"; empty means auto-detect.
	Language string `yaml:"language"`
}

type AIConfig struct {
	Provider      string `yaml:"provider"`
	Model         string `yaml:"model"`
	Endpoint      string `yaml:"endpoint"`
	APIKeyEnv     string `yaml:"api_key_env"`
	RedactSecrets *bool  `yaml:"redact_secrets"`
}

type EngineConfig struct {
	Concurrency    int    `yaml:"concurrency"`
	SnapshotDir    string `yaml:"snapshot_dir"`
	DeadmanTimeout int    `yaml:"deadman_timeout_sec"`
	RulesDir       string `yaml:"rules_dir"`
}

type PluginsConfig struct {
	Directory string   `yaml:"directory"`
	Enabled   []string `yaml:"enabled"`
}

// Default returns the configuration used when no file is present.
func Default() Config {
	return Config{
		AI: AIConfig{Provider: "ollama", APIKeyEnv: "TMAEGIS_AI_API_KEY"},
		Engine: EngineConfig{
			Concurrency:    4,
			SnapshotDir:    "./.tmaegis/snapshots",
			DeadmanTimeout: 60,
			RulesDir:       "./rules",
		},
		Plugins: PluginsConfig{Directory: "./plugins"},
	}
}

// Load reads path over the defaults. A missing file is not an error, so the CLI
// works out of the box; a malformed file is.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
