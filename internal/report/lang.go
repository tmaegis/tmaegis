// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"encoding/json"
	"html"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/tmaegis/tmaegis/internal/i18n"
)

// Lang is the default report language, used when WriteLang gets an empty language. When it is
// empty too, the language is detected from the environment (i18n.Detect). It is only ever read
// while writing, so concurrent WriteLang calls with different languages are safe.
var Lang = ""

// Info carries facts about the scan that the engine's ScanReport does not hold. The CLI sets
// DefaultInfo once at start-up (before any report is written) or passes its own Info to WriteWith.
type Info struct {
	Version  string        // tool version, e.g. "0.1.0"
	RulesDir string        // directory the rules were loaded from
	Duration time.Duration // scan duration (0 = unknown)
	Options  []string      // human-readable scan options, e.g. "--level 2", "--include-manual"
}

// DefaultInfo is used by Write and WriteLang.
var DefaultInfo Info

// resolveLang picks the language tag for one report.
func resolveLang(lang string) string {
	if strings.TrimSpace(lang) == "" {
		lang = Lang
	}
	if strings.TrimSpace(lang) == "" {
		lang = i18n.Detect()
	}
	return i18n.Resolve(lang)
}

// ctx is everything one write needs. It is created per call, so writers share no state.
type ctx struct {
	tag  string
	tr   *i18n.Translator
	info Info
}

func newCtx(lang string, info Info) *ctx {
	tag := resolveLang(lang)
	return &ctx{tag: tag, tr: i18n.New(tag), info: info}
}

// ---- compact dictionary for the browser ---------------------------------------------------

// uiPrefix selects the keys the HTML report needs in the browser. The xlsx/pdf keys stay out.
func uiKey(k string) bool {
	return strings.HasPrefix(k, "report.") && !strings.HasPrefix(k, "report.xlsx.") && !strings.HasPrefix(k, "report.pdf.")
}

var (
	dictOnce sync.Once
	dictJSON template.JS
)

// dictionary returns the JSON the report embeds: en-US in full and, for every other locale, only
// the keys whose text differs once the en-US fallback is filled in. The browser looks a key up in
// the chosen locale first and in en-US second, so the effective dictionary of each of the 16
// locales is complete while the file stays small.
func dictionary() template.JS {
	dictOnce.Do(func() {
		base := map[string]string{}
		for k, v := range i18n.Messages(i18n.Default) {
			if uiKey(k) {
				base[k] = v
			}
		}
		msgs := map[string]map[string]string{i18n.Default: base}
		for _, tag := range i18n.Tags {
			if tag == i18n.Default {
				continue
			}
			d := map[string]string{}
			for k, v := range i18n.Messages(tag) {
				if uiKey(k) && base[k] != v {
					d[k] = v
				}
			}
			msgs[tag] = d
		}
		b, _ := json.Marshal(map[string]interface{}{
			"default": i18n.Default, "tags": i18n.Tags, "names": i18n.Labels(), "m": msgs,
		})
		dictJSON = template.JS(b) // json.Marshal escapes <, > and & so it cannot close the script element
	})
	return dictJSON
}

// ---- template helpers -----------------------------------------------------------------------

func attrEsc(s string) string { return html.EscapeString(s) }

func argsAttr(args []any) string {
	if len(args) < 2 {
		return ""
	}
	m := map[string]any{}
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			m[k] = args[i+1]
		}
	}
	b, _ := json.Marshal(m)
	return ` data-args="` + attrEsc(string(b)) + `"`
}

// funcs returns the template helpers bound to one write's translator.
func (c *ctx) funcs() template.FuncMap {
	return template.FuncMap{
		"dict": func(kv ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(kv); i += 2 {
				m[kv[i].(string)] = kv[i+1]
			}
			return m
		},
		"T": func(key string, args ...any) string { return c.tr.T(key, args...) },
		"N": func(key string, n int, args ...any) string { return c.tr.N(key, n, args...) },
		// A marks an element's text as translatable: <h2 {{A "key"}}>{{T "key"}}</h2>
		"A": func(key string) template.HTMLAttr { return template.HTMLAttr(`data-i18n="` + attrEsc(key) + `"`) },
		// AP is A for text with {placeholders}.
		"AP": func(key string, args ...any) template.HTMLAttr {
			return template.HTMLAttr(`data-i18n="` + attrEsc(key) + `"` + argsAttr(args))
		},
		// AN is A for plural text: n picks the plural form.
		"AN": func(key string, n int, args ...any) template.HTMLAttr {
			return template.HTMLAttr(`data-i18n="` + attrEsc(key) + `" data-n="` + itoa(n) + `"` + argsAttr(args))
		},
		// AA marks a translatable attribute (aria-label, placeholder, title).
		"AA": func(attr, key string) template.HTMLAttr {
			return template.HTMLAttr(`data-i18n-` + attrEsc(attr) + `="` + attrEsc(key) + `"`)
		},
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

type langOption struct{ Tag, Name string }

func langOptions() []langOption {
	out := make([]langOption, 0, len(i18n.Tags))
	for _, t := range i18n.Tags {
		out = append(out, langOption{t, i18n.Label(t)})
	}
	return out
}
