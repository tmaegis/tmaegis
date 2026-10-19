// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package i18n is TMAegis' translation layer, shared by the CLI, the local GUI and the
// reports.
//
// Catalogs are flat JSON files at locales/<tag>/<namespace>.json (cli.json, report.json,
// gui.json, ...). Every file of a locale is merged. en-US is the source of truth: a key
// missing from another locale falls back to en-US, and a key missing everywhere returns
// the key itself so gaps are visible rather than blank.
//
// Values may contain {name} placeholders. Plural forms are separate keys with a CLDR
// category suffix: "scan.rules.one", "scan.rules.few", "scan.rules.other" (use N).
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed locales
var localesFS embed.FS

// Default is the source language every other catalog falls back to.
const Default = "en-US"

// Tags lists the supported locales (BCP 47). Serbian ships in both scripts.
var Tags = []string{
	"en-US", "en-GB", "vi", "zh-Hans", "zh-Hant", "es-419", "fr", "de",
	"ja", "ko", "ru", "tr", "pt-BR", "id", "sr-Latn", "sr-Cyrl",
}

// Names are the endonyms shown in language pickers.
var Names = map[string]string{
	"en-US": "English (US)", "en-GB": "English (UK)", "vi": "Tiếng Việt", "zh-Hans": "简体中文",
	"zh-Hant": "繁體中文", "es-419": "Español (Latinoamérica)", "fr": "Français", "de": "Deutsch",
	"ja": "日本語", "ko": "한국어", "ru": "Русский", "tr": "Türkçe", "pt-BR": "Português (Brasil)",
	"id": "Bahasa Indonesia", "sr-Latn": "Srpski (latinica)", "sr-Cyrl": "Српски (ћирилица)",
}

var (
	mu     sync.Mutex
	loaded = map[string]map[string]string{}
)

// catalog returns the merged messages of one locale (nil when it has none yet).
func catalog(tag string) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := loaded[tag]; ok {
		return c
	}
	c := map[string]string{}
	entries, err := fs.ReadDir(localesFS, path.Join("locales", tag))
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := localesFS.ReadFile(path.Join("locales", tag, e.Name()))
			if err != nil {
				continue
			}
			var m map[string]string
			if json.Unmarshal(b, &m) == nil {
				for k, v := range m {
					c[k] = v
				}
			}
		}
	}
	loaded[tag] = c
	return c
}

// Keys returns the sorted keys of a locale (used by tooling and tests).
func Keys(tag string) []string {
	var ks []string
	for k := range catalog(tag) {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Messages returns a copy of a locale's merged catalog with the en-US fallbacks filled in,
// for front ends (the GUI and the HTML report) that translate in the browser.
func Messages(tag string) map[string]string {
	out := map[string]string{}
	for k, v := range catalog(Default) {
		out[k] = v
	}
	for _, t := range chainFor(Resolve(tag)) {
		for k, v := range catalog(t) {
			out[k] = v
		}
	}
	return out
}

// chainFor lists the locales to consult, most specific last (so later entries override).
func chainFor(tag string) []string {
	switch tag {
	case "en-GB":
		return []string{"en-US", "en-GB"}
	case "":
		return []string{Default}
	}
	return []string{Default, tag}
}

// Translator looks keys up for one locale.
type Translator struct {
	tag  string
	msgs map[string]string
}

// New builds a Translator for tag (any BCP 47 / POSIX locale string is accepted).
func New(tag string) *Translator {
	tag = Resolve(tag)
	return &Translator{tag: tag, msgs: Messages(tag)}
}

// Tag is the resolved locale, e.g. "zh-Hans".
func (t *Translator) Tag() string { return t.tag }

// Has reports whether key exists (in this locale or en-US).
func (t *Translator) Has(key string) bool { _, ok := t.msgs[key]; return ok }

// T translates key. args are alternating placeholder names and values:
// t.T("scan.done", "count", 5) fills "{count}".
func (t *Translator) T(key string, args ...any) string {
	s, ok := t.msgs[key]
	if !ok {
		return key
	}
	return format(s, args)
}

// N translates a plural key for n. It looks up key.<category> for the locale's CLDR plural
// category of n, then key.other, then key. "{n}" is filled in automatically.
func (t *Translator) N(key string, n int, args ...any) string {
	cat := pluralCategory(t.tag, n)
	for _, k := range []string{key + "." + cat, key + ".other", key} {
		if s, ok := t.msgs[k]; ok {
			return format(s, append([]any{"n", n}, args...))
		}
	}
	return key
}

func format(s string, args []any) string {
	if len(args) == 0 || !strings.Contains(s, "{") {
		return s
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		pairs = append(pairs, "{"+fmt.Sprint(args[i])+"}", fmt.Sprint(args[i+1]))
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// PluralCategories are the CLDR categories a locale uses for whole numbers.
func PluralCategories(tag string) []string {
	switch Resolve(tag) {
	case "ja", "ko", "zh-Hans", "zh-Hant", "vi", "id":
		return []string{"other"}
	case "ru":
		return []string{"one", "few", "many", "other"}
	case "sr-Latn", "sr-Cyrl":
		return []string{"one", "few", "other"}
	}
	return []string{"one", "other"}
}

func pluralCategory(tag string, n int) string {
	if n < 0 {
		n = -n
	}
	switch tag {
	case "ja", "ko", "zh-Hans", "zh-Hant", "vi", "id":
		return "other"
	case "fr", "pt-BR":
		if n == 0 || n == 1 {
			return "one"
		}
		return "other"
	case "ru", "sr-Latn", "sr-Cyrl":
		m10, m100 := n%10, n%100
		switch {
		case m10 == 1 && m100 != 11:
			return "one"
		case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
			return "few"
		case tag == "ru":
			return "many"
		}
		return "other"
	}
	if n == 1 {
		return "one"
	}
	return "other"
}

// Resolve maps any locale string ("en_GB.UTF-8", "zh-TW", "pt", "sr-Cyrl-RS") to a supported tag.
func Resolve(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	if s == "" || strings.EqualFold(s, "C") || strings.EqualFold(s, "POSIX") {
		return Default
	}
	for _, t := range Tags {
		if strings.EqualFold(t, s) {
			return t
		}
	}
	parts := strings.Split(strings.ToLower(s), "-")
	lang, script, region := parts[0], "", ""
	for _, p := range parts[1:] {
		switch {
		case len(p) == 4:
			script = p
		case len(p) == 2 || len(p) == 3:
			region = p
		}
	}
	switch lang {
	case "en":
		switch region {
		case "gb", "au", "nz", "ie", "in", "za", "sg", "hk", "my", "ph", "pk":
			return "en-GB"
		}
		return "en-US"
	case "zh":
		if script == "hant" || region == "tw" || region == "hk" || region == "mo" {
			return "zh-Hant"
		}
		return "zh-Hans"
	case "sr":
		if script == "cyrl" {
			return "sr-Cyrl"
		}
		return "sr-Latn" // Latin by default: it is what Serbian IT users overwhelmingly type
	case "es":
		return "es-419"
	case "pt":
		return "pt-BR"
	case "in": // legacy code for Indonesian
		return "id"
	case "vi", "fr", "de", "ja", "ko", "ru", "tr", "id":
		return lang
	}
	return Default
}

// Detect picks the UI language: TMAEGIS_LANG, then the POSIX locale variables, then the
// operating system's user locale (Windows), then en-US.
func Detect() string {
	for _, k := range []string{"TMAEGIS_LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} { // LANGUAGE wins over LANG, as in gettext
		if v := os.Getenv(k); v != "" && !strings.EqualFold(v, "C") && !strings.EqualFold(v, "POSIX") {
			if i := strings.Index(v, ":"); i > 0 { // LANGUAGE=fr:en
				v = v[:i]
			}
			return Resolve(v)
		}
	}
	if v := systemLocale(); v != "" {
		return Resolve(v)
	}
	return Default
}

var (
	curMu sync.RWMutex
	cur   = New(Default)
)

// SetLang sets the process-wide translator used by the package-level helpers.
func SetLang(tag string) {
	curMu.Lock()
	cur = New(tag)
	curMu.Unlock()
}

// Lang returns the process-wide locale tag.
func Lang() string { curMu.RLock(); defer curMu.RUnlock(); return cur.tag }

// T translates with the process-wide locale.
func T(key string, args ...any) string {
	curMu.RLock()
	t := cur
	curMu.RUnlock()
	return t.T(key, args...)
}

// N translates a plural key with the process-wide locale.
func N(key string, n int, args ...any) string {
	curMu.RLock()
	t := cur
	curMu.RUnlock()
	return t.N(key, n, args...)
}
