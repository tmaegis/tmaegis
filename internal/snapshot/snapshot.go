// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"github.com/tmaegis/tmaegis/internal/rules"
)

type FileBackup struct {
	OriginalPath string `json:"original_path"`
	Content      []byte `json:"content"`
	Mode         uint32 `json:"mode"`
	// Absent means the file did not exist before; rollback deletes it.
	Absent bool `json:"absent,omitempty"`
}

type StateSnapshot struct {
	ID        string       `json:"id"`
	CreatedAt time.Time    `json:"created_at"`
	Files     []FileBackup `json:"files"`
}

type Manager struct {
	BaseDir string
}

func NewManager(baseDir string) *Manager {
	return &Manager{BaseDir: baseDir}
}

func (m *Manager) CreateSnapshot(ruleList []rules.Rule) (string, error) {
	if err := os.MkdirAll(m.BaseDir, 0700); err != nil {
		return "", err
	}

	snapID := fmt.Sprintf("snap-%d", time.Now().UTC().UnixNano())
	snap := StateSnapshot{
		ID:        snapID,
		CreatedAt: time.Now().UTC(),
	}

	seen := make(map[string]bool)
	for _, r := range ruleList {
		for _, fpath := range r.Remediation.BackupFiles {
			if seen[fpath] {
				continue
			}
			seen[fpath] = true
			info, err := os.Stat(fpath)
			if os.IsNotExist(err) {
				snap.Files = append(snap.Files, FileBackup{OriginalPath: fpath, Absent: true})
				continue
			}
			if err != nil {
				return "", lerr.Wrap("cli.snapshot.err.backup", err, "path", fpath)
			}
			data, err := os.ReadFile(fpath)
			if err != nil {
				return "", lerr.Wrap("cli.snapshot.err.backup", err, "path", fpath)
			}
			snap.Files = append(snap.Files, FileBackup{
				OriginalPath: fpath,
				Content:      data,
				Mode:         uint32(info.Mode()),
			})
		}
	}

	payload, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}

	outPath := filepath.Join(m.BaseDir, snapID+".json")
	if err := os.WriteFile(outPath, payload, 0600); err != nil {
		return "", err
	}

	return snapID, nil
}

func (m *Manager) GetLatestSnapshotID() (string, error) {
	entries, err := os.ReadDir(m.BaseDir)
	if err != nil {
		return "", err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			ids = append(ids, e.Name()[:len(e.Name())-5])
		}
	}
	if len(ids) == 0 {
		return "", lerr.New("cli.snapshot.err.none")
	}
	sort.Strings(ids)
	return ids[len(ids)-1], nil
}

func (m *Manager) Rollback(snapID string) error {
	snapPath := filepath.Join(m.BaseDir, snapID+".json")
	data, err := os.ReadFile(snapPath)
	if err != nil {
		return lerr.Wrap("cli.snapshot.err.read", err, "id", snapID)
	}

	var snap StateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}

	for _, fb := range snap.Files {
		if fb.Absent {
			err := os.Remove(fb.OriginalPath)
			if err == nil {
				fmt.Println(i18n.T("cli.snapshot.removed", "path", fb.OriginalPath))
			} else if !os.IsNotExist(err) {
				return lerr.Wrap("cli.snapshot.err.remove", err, "path", fb.OriginalPath)
			}
			continue
		}
		if err := os.WriteFile(fb.OriginalPath, fb.Content, os.FileMode(fb.Mode)); err != nil {
			return lerr.Wrap("cli.snapshot.err.restore", err, "path", fb.OriginalPath)
		}
		fmt.Println(i18n.T("cli.snapshot.restored", "path", fb.OriginalPath))
	}
	return nil
}

// Info summarises a stored snapshot for `rollback --list`.
type Info struct {
	ID        string
	CreatedAt time.Time
	Files     int
}

// List returns stored snapshots, newest first.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.BaseDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var snap StateSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue // skip corrupt snapshots rather than hiding the rest
		}
		out = append(out, Info{ID: snap.ID, CreatedAt: snap.CreatedAt, Files: len(snap.Files)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}
