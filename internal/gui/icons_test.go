// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/tmaegis/tmaegis/internal/i18n"
)

// spriteSymbols parses the embedded sprite and returns its symbol ids.
func spriteSymbols(t *testing.T) map[string]bool {
	t.Helper()
	b, err := assetsFS.ReadFile("assets/" + spriteName)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		XMLName xml.Name `xml:"svg"`
		Symbols []struct {
			ID      string `xml:"id,attr"`
			ViewBox string `xml:"viewBox,attr"`
			Inner   string `xml:",innerxml"`
		} `xml:"symbol"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("sprite is not valid XML: %v", err)
	}
	ids := map[string]bool{}
	for _, s := range doc.Symbols {
		if ids[s.ID] {
			t.Errorf("duplicate symbol %q", s.ID)
		}
		if s.ViewBox != "0 0 24 24" || strings.TrimSpace(s.Inner) == "" {
			t.Errorf("symbol %q: viewBox %q, empty=%v", s.ID, s.ViewBox, strings.TrimSpace(s.Inner) == "")
		}
		ids[s.ID] = true
	}
	return ids
}

// Every target of the platform catalogue (all 96 of the website's lib/targets.ts) has a logo, glyph or
// monogram in the sprite, and every logo colour belongs to a symbol that exists.
func TestPlatformIconsCoverCatalog(t *testing.T) {
	if len(platformCatalog) != 96 {
		t.Errorf("platform catalogue has %d targets, want the website's 96", len(platformCatalog))
	}
	syms := spriteSymbols(t)
	ic := platformIcons()
	for id := range platformCatalog {
		sym, ok := ic.Targets[id]
		if !ok {
			t.Errorf("target %q has no icon (re-run tools/gen-platform-icons.mjs)", id)
			continue
		}
		if !syms[sym] {
			t.Errorf("target %q: symbol %q is not in the sprite", id, sym)
		}
		// a monogram is drawn framed next to text: that variant must exist too
		if strings.HasPrefix(sym, "m-") && !syms["mf-"+sym[2:]] {
			t.Errorf("target %q: framed monogram %q is missing", id, "mf-"+sym[2:])
		}
		if ic.Names[id] != platformCatalog[id].name {
			t.Errorf("target %q: name %q", id, ic.Names[id])
		}
	}
	hex := regexp.MustCompile(`^[0-9A-F]{6}$`)
	for slug, b := range ic.Brands {
		if !syms[slug] || !hex.MatchString(b.Hex) || b.Title == "" {
			t.Errorf("brand %q: in sprite=%v hex=%q title=%q", slug, syms[slug], b.Hex, b.Title)
		}
	}
	if len(ic.Brands) < 50 {
		t.Errorf("only %d logos", len(ic.Brands))
	}
	// full-colour logos are never recoloured; a chip names the themes it is drawn in
	chip := regexp.MustCompile(`^(dark|light)( light)?$`)
	for slug, b := range ic.Brands {
		if b.Color && (b.Dark || b.Light) || b.Chip != "" && (!b.Color || !chip.MatchString(b.Chip)) {
			t.Errorf("brand %q: color=%v chip=%q dark=%v light=%v", slug, b.Color, b.Chip, b.Dark, b.Light)
		}
	}
	// the website's real logos, not monograms, for the platforms that have one (same table as web-portal)
	for id, want := range map[string]string{"windows": "l-microsoft-windows-icon", "azure": "l-microsoft-azure", "aws": "l-aws", "edge": "l-microsoft-edge", "aix": "ibm", "oracle-db": "oracle", "mssql": "v-microsoft-sql-server", "exchange": "v-microsoft-exchange", "gke": "v-google-kubernetes-engine", "checkpoint": "v-check-point"} {
		if ic.Targets[id] != want {
			t.Errorf("target %q: symbol %q, want %q", id, ic.Targets[id], want)
		}
	}
	if !regexp.MustCompile(`^/static/platform-icons\.svg\?v=[0-9a-f]{12}$`).MatchString(ic.Sprite) {
		t.Errorf("sprite URL %q", ic.Sprite)
	}
}

func TestPlatformSpriteServedAndCached(t *testing.T) {
	e := newEnv(t, false)
	ic := platformIcons()

	// the session hands the UI the versioned sprite URL and the icon table
	c, body := e.do(t, "GET", "/api/session", nil, nil)
	var sess struct {
		Icons struct {
			Sprite  string            `json:"sprite"`
			Targets map[string]string `json:"targets"`
			Names   map[string]string `json:"names"`
		} `json:"icons"`
	}
	if c != 200 || json.Unmarshal(body, &sess) != nil || sess.Icons.Sprite != ic.Sprite || sess.Icons.Targets["postgres"] != "postgresql" || sess.Icons.Names["postgres"] != "PostgreSQL" {
		t.Fatalf("session icons: %d %+v", c, sess.Icons.Sprite)
	}

	get := func(path string, mod func(*http.Request)) *http.Response {
		req, _ := http.NewRequest("GET", e.base+path, nil)
		req.Header.Set("Authorization", "Bearer "+e.s.Token())
		if mod != nil {
			mod(req)
		}
		resp, err := e.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	r := get(ic.Sprite, nil)
	if r.StatusCode != 200 || r.Header.Get("Content-Type") != "image/svg+xml" || r.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("sprite: %d %q", r.StatusCode, r.Header.Get("Content-Type"))
	}
	if cc := r.Header.Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
		t.Errorf("versioned sprite Cache-Control %q", cc)
	}
	etag := r.Header.Get("ETag")
	if etag != `"`+ic.hash+`"` {
		t.Errorf("ETag %q", etag)
	}
	// an unversioned or stale URL revalidates instead of being cached for good
	r = get("/static/"+spriteName+"?v=old", nil)
	if cc := r.Header.Get("Cache-Control"); r.StatusCode != 200 || cc != "private, no-cache" {
		t.Errorf("stale sprite URL: %d %q", r.StatusCode, cc)
	}
	r = get("/static/"+spriteName, func(q *http.Request) { q.Header.Set("If-None-Match", etag) })
	if r.StatusCode != http.StatusNotModified {
		t.Errorf("If-None-Match: %d", r.StatusCode)
	}
	// other assets keep the no-store default, and the sprite still needs the session token
	if r = get("/static/app.js", nil); r.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("app.js Cache-Control %q", r.Header.Get("Cache-Control"))
	}
	if r = get(ic.Sprite, func(q *http.Request) { q.Header.Del("Authorization") }); r.StatusCode != 401 {
		t.Errorf("sprite without token: %d", r.StatusCode)
	}
}

// The About page carries the trademark notice for the logos.
func TestBrandNoticeMessage(t *testing.T) {
	b, err := assetsFS.ReadFile("assets/app.js")
	if err != nil || !strings.Contains(string(b), "'gui.about.brands'") {
		t.Fatal("About does not show gui.about.brands")
	}
	want := "Product names, logos and brands are property of their respective owners and are used only to identify supported platforms."
	if got := i18n.New("en-US").T("gui.about.brands"); got != want {
		t.Errorf("gui.about.brands = %q", got)
	}
}
