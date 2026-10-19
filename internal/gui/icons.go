// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

//go:generate node tools/gen-platform-icons.mjs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

// Platform logos come from the website's icon set (web-portal/lib/platform-icons.ts), converted by
// tools/gen-platform-icons.mjs into an SVG <symbol> sprite (assets/platform-icons.svg) and a target ->
// symbol table (assets/platform-icons.json). The sprite is fetched once and cached by the browser: its URL
// carries a content hash, so it can be cached for good. Product names, logos and brands are property of
// their respective owners and are used only to identify supported platforms.

const spriteName = "platform-icons.svg"

type brandIcon struct {
	Title string `json:"title"`
	Hex   string `json:"hex"`
	Color bool   `json:"color,omitempty"` // a full-colour logo, drawn in its own colours and never recoloured
	Chip  string `json:"chip,omitempty"`  // themes ("dark", "light") in which a full-colour logo sits on a contrasting chip
	Wide  bool   `json:"wide,omitempty"`  // a wordmark much wider than tall, drawn larger on a tile
	Dark  bool   `json:"dark,omitempty"`  // the brand colour of a single-colour mark reaches 3:1 on the dark theme's backgrounds
	Light bool   `json:"light,omitempty"` // ... and on the light theme's
}

type iconSet struct {
	Sprite  string               `json:"sprite"`  // URL of the sprite, versioned by content hash
	Targets map[string]string    `json:"targets"` // target id -> symbol id
	Brands  map[string]brandIcon `json:"brands"`  // symbol id of a logo -> its colour
	Names   map[string]string    `json:"names"`   // target id -> product name (the text shown beside an icon)
	hash    string
}

var (
	iconsOnce sync.Once
	iconsData *iconSet
)

// platformIcons loads the embedded icon table once.
func platformIcons() *iconSet {
	iconsOnce.Do(func() {
		set := &iconSet{Targets: map[string]string{}, Brands: map[string]brandIcon{}, Names: map[string]string{}}
		if b, err := assetsFS.ReadFile("assets/platform-icons.json"); err == nil {
			_ = json.Unmarshal(b, set)
		}
		sprite, _ := assetsFS.ReadFile("assets/" + spriteName)
		sum := sha256.Sum256(sprite)
		set.hash = hex.EncodeToString(sum[:])[:12]
		set.Sprite = "/static/" + spriteName + "?v=" + set.hash
		for id, m := range platformCatalog {
			set.Names[id] = m.name
		}
		iconsData = set
	})
	return iconsData
}
