// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var wasmMagic = []byte("\x00asm\x01\x00\x00\x00")

func TestInstallLocalAndChecksum(t *testing.T) {
	src := filepath.Join(t.TempDir(), "notify.sh")
	os.WriteFile(src, []byte("#!/bin/sh\necho hi\n"), 0o600)
	m := NewManager(filepath.Join(t.TempDir(), "plugins"))

	if _, _, err := m.Install(context.Background(), src, InstallOptions{SHA256: "deadbeef"}); err == nil {
		t.Fatal("checksum mismatch must abort the install")
	}
	if list, _ := m.Discover(); len(list) != 0 {
		t.Fatal("nothing must be installed after a failed checksum")
	}

	sum := sha256.Sum256([]byte("#!/bin/sh\necho hi\n"))
	full, got, err := m.Install(context.Background(), src, InstallOptions{SHA256: hex.EncodeToString(sum[:])})
	if err != nil || got != hex.EncodeToString(sum[:]) {
		t.Fatalf("install: %v", err)
	}
	if filepath.Base(full) != "tmaegis-plugin-notify.sh" {
		t.Fatalf("expected prefixed name, got %s", full)
	}
	// Windows has no executable bit (os.Stat reports 0666 for files that are not .exe/.bat/.cmd)
	if info, _ := os.Stat(full); runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Fatal("executable plugin must be executable")
	}
	list, _ := m.Discover()
	if len(list) != 1 || list[0].Name != "notify" {
		t.Fatalf("discover: %v", list)
	}
	if err := m.Remove("notify"); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove("notify"); err == nil {
		t.Fatal("removing twice must error")
	}
}

func TestInstallRejectsBadWasmAndUnsafeNames(t *testing.T) {
	d := t.TempDir()
	bad := filepath.Join(d, "x.wasm")
	os.WriteFile(bad, []byte("nope"), 0o600)
	m := NewManager(filepath.Join(d, "p"))
	if _, _, err := m.Install(context.Background(), bad, InstallOptions{}); err == nil {
		t.Fatal("non-wasm content with .wasm name must be rejected")
	}
	if _, err := m.destName("..", false); err == nil {
		t.Fatal("unsafe name must be rejected")
	}
	if _, err := m.destName(".hidden", false); err == nil {
		t.Fatal("dotfile name must be rejected")
	}
}

func TestInstallFromHTTPAndGitHub(t *testing.T) {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/dl/slack.wasm", func(w http.ResponseWriter, r *http.Request) { w.Write(wasmMagic) })
	mux.HandleFunc("/repos/acme/plugin-x/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"assets":[{"name":"x-%s-%s","browser_download_url":"%s/dl/native"},{"name":"x.wasm","browser_download_url":"%s/dl/slack.wasm"}]}`,
			runtime.GOOS, runtime.GOARCH, srv.URL, srv.URL)
	})
	mux.HandleFunc("/repos/acme/plugin-y/releases/tags/v1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"assets":[{"name":"y-%s-%s","browser_download_url":"%s/dl/native"}]}`, runtime.GOOS, runtime.GOARCH, srv.URL)
	})
	mux.HandleFunc("/dl/native", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("#!/bin/sh\n")) })
	srv = httptest.NewServer(mux)
	defer srv.Close()
	old := GitHubAPI
	GitHubAPI = srv.URL
	defer func() { GitHubAPI = old }()

	m := NewManager(t.TempDir())
	if p, _, err := m.Install(context.Background(), srv.URL+"/dl/slack.wasm", InstallOptions{}); err != nil || filepath.Base(p) != "slack.wasm" {
		t.Fatalf("http install: %v %s", err, p)
	}
	if p, _, err := m.Install(context.Background(), "github.com/acme/plugin-x", InstallOptions{}); err != nil || filepath.Base(p) != "x.wasm" {
		t.Fatalf("github should prefer wasm: %v %s", err, p)
	}
	if p, _, err := m.Install(context.Background(), "github.com/acme/plugin-y@v1", InstallOptions{}); err != nil || filepath.Base(p) != "tmaegis-plugin-y-"+runtime.GOOS+"-"+runtime.GOARCH {
		t.Fatalf("github native fallback: %v %s", err, p)
	}
	if _, _, err := m.Install(context.Background(), "github.com/acme/missing", InstallOptions{}); err == nil {
		t.Fatal("404 must error")
	}
	if _, _, err := m.Install(context.Background(), "github.com/bad", InstallOptions{}); err == nil {
		t.Fatal("malformed github source must error")
	}
}
