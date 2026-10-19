// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaegis/tmaegis/internal/rules"
)

func TestSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sshd_config")
	os.WriteFile(target, []byte("original\n"), 0o640)

	m := NewManager(filepath.Join(dir, "snaps"))
	rs := []rules.Rule{{ID: "A", Remediation: rules.RemediateSpec{BackupFiles: []string{target, target, filepath.Join(dir, "absent")}}}}
	id1, err := m.CreateSnapshot(rs)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := m.CreateSnapshot(rs)
	if err != nil || id1 == id2 {
		t.Fatalf("snapshot ids must be unique: %s %s %v", id1, id2, err)
	}
	if latest, _ := m.GetLatestSnapshotID(); latest != id2 {
		t.Fatalf("latest = %s, want %s", latest, id2)
	}

	os.WriteFile(target, []byte("tampered\n"), 0o600)
	if err := m.Rollback(id1); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "original\n" {
		t.Fatalf("not restored: %q", b)
	}
}

func TestRollbackErrors(t *testing.T) {
	m := NewManager(t.TempDir())
	if _, err := m.GetLatestSnapshotID(); err == nil {
		t.Fatal("expected error with no snapshots")
	}
	if err := m.Rollback("snap-missing"); err == nil {
		t.Fatal("expected error for unknown snapshot")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(filepath.Join(dir, "snaps"))
	if l, err := m.List(); err != nil || len(l) != 0 {
		t.Fatalf("empty dir: %v %v", l, err)
	}
	f := filepath.Join(dir, "f")
	os.WriteFile(f, []byte("x"), 0o600)
	rs := []rules.Rule{{Remediation: rules.RemediateSpec{BackupFiles: []string{f}}}}
	a, _ := m.CreateSnapshot(rs)
	b, _ := m.CreateSnapshot(rs)
	os.WriteFile(filepath.Join(m.BaseDir, "snap-corrupt.json"), []byte("{"), 0o600)
	l, err := m.List()
	if err != nil || len(l) != 2 || l[0].ID != b || l[1].ID != a || l[0].Files != 1 {
		t.Fatalf("list: %+v %v", l, err)
	}
}

func TestRollbackRemovesFilesCreatedAfterSnapshot(t *testing.T) {
	dir := t.TempDir()
	created := filepath.Join(dir, "zz-dropin.conf")
	neverCreated := filepath.Join(dir, "other-distro-path.conf")
	m := NewManager(filepath.Join(dir, "snaps"))
	id, err := m.CreateSnapshot([]rules.Rule{{Remediation: rules.RemediateSpec{BackupFiles: []string{created, neverCreated}}}})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(created, []byte("ServerTokens Prod\n"), 0o644) // what the remediation wrote
	if err := m.Rollback(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatal("a file that did not exist before the snapshot must be removed on rollback")
	}
}
