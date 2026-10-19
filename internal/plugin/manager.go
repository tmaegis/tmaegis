// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PluginInfo struct {
	Name string
	Type string // "executable" or "wasm"
	Path string
}

type Manager struct {
	Dir string
}

func NewManager(dir string) *Manager {
	return &Manager{Dir: dir}
}

func (m *Manager) Discover() ([]PluginInfo, error) {
	var list []PluginInfo
	entries, err := os.ReadDir(m.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		fullPath := filepath.Join(m.Dir, name)
		if strings.HasSuffix(name, ".wasm") {
			list = append(list, PluginInfo{
				Name: strings.TrimSuffix(name, ".wasm"),
				Type: "wasm",
				Path: fullPath,
			})
		} else if strings.HasPrefix(name, "tmaegis-plugin-") {
			shortName := strings.TrimPrefix(name, "tmaegis-plugin-")
			shortName = strings.TrimSuffix(shortName, ".sh")
			shortName = strings.TrimSuffix(shortName, ".py")
			shortName = strings.TrimSuffix(shortName, ".exe")
			list = append(list, PluginInfo{
				Name: shortName,
				Type: "executable",
				Path: fullPath,
			})
		}
	}
	return list, nil
}

// Execute pipes the JSON scan report to the plugin via STDIN and prints its STDOUT.
func (m *Manager) Execute(ctx context.Context, pluginName string, payload interface{}) error {
	plugins, err := m.Discover()
	if err != nil {
		return err
	}

	var target *PluginInfo
	for _, p := range plugins {
		if p.Name == pluginName {
			target = &p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("plugin '%s' not found in %s", pluginName, m.Dir)
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var out []byte
	if target.Type == "wasm" {
		out, err = RunWasm(ctx, target.Path, jsonBytes)
	} else {
		cmd := exec.CommandContext(ctx, target.Path)
		cmd.Stdin = bytes.NewReader(jsonBytes)
		out, err = cmd.CombinedOutput()
	}
	fmt.Fprintf(os.Stderr, "  [Plugin %s Output]: %s\n", target.Name, strings.TrimSpace(string(out)))
	return err
}
