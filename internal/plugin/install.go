// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// GitHubAPI is the base URL used to resolve `github.com/owner/repo` sources (overridable in tests).
var GitHubAPI = "https://api.github.com"

const maxPluginSize = 64 << 20

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// InstallOptions controls Install.
type InstallOptions struct {
	// SHA256, when set, must match the downloaded file or the install is aborted.
	SHA256 string
}

// Install fetches a plugin into dir and returns its final path and SHA-256.
// source may be a local file, an http(s) URL, or github.com/owner/repo[@tag], which
// downloads the matching release asset (preferring portable .wasm over native binaries).
func (m *Manager) Install(ctx context.Context, source string, opts InstallOptions) (string, string, error) {
	data, filename, err := fetch(ctx, source)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	if opts.SHA256 != "" && !strings.EqualFold(opts.SHA256, hexSum) {
		return "", "", fmt.Errorf("checksum mismatch: expected %s, got %s", opts.SHA256, hexSum)
	}

	isWasm := strings.HasSuffix(filename, ".wasm")
	if isWasm && !bytes.HasPrefix(data, []byte("\x00asm")) {
		return "", "", fmt.Errorf("%s is not a valid WebAssembly module", filename)
	}
	dest, err := m.destName(filename, isWasm)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return "", "", err
	}
	full := filepath.Join(m.Dir, dest)
	mode := os.FileMode(0o755)
	if isWasm {
		mode = 0o644
	}
	tmp := full + ".part"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmp, full); err != nil {
		os.Remove(tmp)
		return "", "", err
	}
	return full, hexSum, nil
}

// Remove deletes an installed plugin by its short name.
func (m *Manager) Remove(name string) error {
	list, err := m.Discover()
	if err != nil {
		return err
	}
	for _, p := range list {
		if p.Name == name {
			return os.Remove(p.Path)
		}
	}
	return fmt.Errorf("plugin '%s' is not installed", name)
}

// destName normalises the stored filename so Discover can find it, and rejects path tricks.
func (m *Manager) destName(filename string, isWasm bool) (string, error) {
	base := path.Base(filepath.ToSlash(filename))
	if !validName.MatchString(base) {
		return "", fmt.Errorf("unsafe plugin filename %q", filename)
	}
	if isWasm || strings.HasPrefix(base, "tmaegis-plugin-") {
		return base, nil
	}
	return "tmaegis-plugin-" + base, nil
}

func fetch(ctx context.Context, source string) ([]byte, string, error) {
	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		u, err := url.Parse(source)
		if err != nil {
			return nil, "", err
		}
		data, err := httpGet(ctx, source)
		return data, path.Base(u.Path), err
	case strings.HasPrefix(source, "github.com/"):
		return fetchGitHub(ctx, strings.TrimPrefix(source, "github.com/"))
	default:
		data, err := os.ReadFile(source)
		return data, filepath.Base(source), err
	}
}

func httpGet(ctx context.Context, u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream, */*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPluginSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPluginSize {
		return nil, fmt.Errorf("plugin exceeds %d MiB limit", maxPluginSize>>20)
	}
	return data, nil
}

type ghRelease struct {
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchGitHub(ctx context.Context, spec string) ([]byte, string, error) {
	repo, tag, _ := strings.Cut(spec, "@")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, "", fmt.Errorf("expected github.com/owner/repo[@tag], got %q", spec)
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", GitHubAPI, parts[0], parts[1])
	if tag != "" {
		endpoint = fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", GitHubAPI, parts[0], parts[1], tag)
	}
	body, err := httpGet(ctx, endpoint)
	if err != nil {
		return nil, "", err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, "", err
	}
	name, assetURL := pickAsset(rel)
	if assetURL == "" {
		return nil, "", fmt.Errorf("no installable asset (.wasm or *%s-%s*) in %s", runtime.GOOS, runtime.GOARCH, repo)
	}
	data, err := httpGet(ctx, assetURL)
	return data, name, err
}

// pickAsset prefers a portable .wasm asset, falling back to a native build for this platform.
func pickAsset(rel ghRelease) (string, string) {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".wasm") {
			return a.Name, a.URL
		}
	}
	plat := runtime.GOOS + "-" + runtime.GOARCH
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, plat) && !strings.HasSuffix(a.Name, ".sha256") {
			return a.Name, a.URL
		}
	}
	return "", ""
}
