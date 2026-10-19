// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package i18n

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	cases := map[string]string{
		"": "en-US", "C": "en-US", "en": "en-US", "en_GB.UTF-8": "en-GB", "en-AU": "en-GB", "vi_VN.UTF-8": "vi",
		"zh_CN.UTF-8": "zh-Hans", "zh-TW": "zh-Hant", "zh-Hant-HK": "zh-Hant", "zh": "zh-Hans", "es": "es-419", "es_ES": "es-419",
		"pt": "pt-BR", "pt_PT": "pt-BR", "sr": "sr-Latn", "sr-RS": "sr-Latn", "sr-Cyrl-RS": "sr-Cyrl", "sr_RS@latin": "sr-Latn",
		"in": "id", "id-ID": "id", "ja_JP.eucJP": "ja", "ko-KR": "ko", "ru": "ru", "tr-TR": "tr", "fr-CA": "fr", "de-AT": "de",
		"xx": "en-US", "ar": "en-US", "SR-LATN": "sr-Latn",
	}
	for in, want := range cases {
		if got := Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
	for _, tag := range Tags {
		if Resolve(tag) != tag {
			t.Errorf("tag %q does not resolve to itself", tag)
		}
		if Names[tag] == "" {
			t.Errorf("no endonym for %q", tag)
		}
	}
}

func TestDetect(t *testing.T) {
	for _, k := range []string{"TMAEGIS_LANG", "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "vi_VN.UTF-8")
	if got := Detect(); got != "vi" {
		t.Fatalf("LANG: %s", got)
	}
	t.Setenv("LANGUAGE", "fr:en")
	if got := Detect(); got != "fr" {
		t.Fatalf("LANGUAGE: %s", got)
	}
	t.Setenv("TMAEGIS_LANG", "ja")
	if got := Detect(); got != "ja" {
		t.Fatalf("override: %s", got)
	}
}

func TestPluralCategories(t *testing.T) {
	cases := []struct {
		tag  string
		n    int
		want string
	}{
		{"en-US", 1, "one"}, {"en-US", 0, "other"}, {"en-US", 5, "other"}, {"fr", 0, "one"}, {"fr", 2, "other"},
		{"ru", 1, "one"}, {"ru", 21, "one"}, {"ru", 11, "many"}, {"ru", 3, "few"}, {"ru", 13, "many"}, {"ru", 5, "many"}, {"ru", 22, "few"},
		{"sr-Latn", 1, "one"}, {"sr-Latn", 4, "few"}, {"sr-Latn", 11, "other"}, {"sr-Cyrl", 5, "other"},
		{"ja", 1, "other"}, {"zh-Hans", 3, "other"}, {"vi", 1, "other"}, {"id", 2, "other"}, {"de", 1, "one"}, {"tr", 1, "one"},
	}
	for _, c := range cases {
		if got := pluralCategory(c.tag, c.n); got != c.want {
			t.Errorf("%s %d = %s, want %s", c.tag, c.n, got, c.want)
		}
	}
}

func TestTranslatorFallbackAndFormat(t *testing.T) {
	mu.Lock()
	loaded["en-US"] = map[string]string{"a": "Hello {name}", "n.one": "{n} rule", "n.other": "{n} rules", "only.en": "English only"}
	loaded["fr"] = map[string]string{"a": "Bonjour {name}", "n.one": "{n} règle", "n.other": "{n} règles"}
	loaded["en-GB"] = map[string]string{"a": "Hello {name}!"}
	mu.Unlock()
	defer func() { mu.Lock(); delete(loaded, "en-US"); delete(loaded, "fr"); delete(loaded, "en-GB"); mu.Unlock() }()

	fr := New("fr")
	if got := fr.T("a", "name", "Tommy"); got != "Bonjour Tommy" {
		t.Fatal(got)
	}
	if got := fr.T("only.en"); got != "English only" {
		t.Fatalf("must fall back to en-US: %s", got)
	}
	if got := fr.T("missing.key"); got != "missing.key" {
		t.Fatalf("missing keys return the key: %s", got)
	}
	if fr.N("n", 0) != "0 règle" || fr.N("n", 2) != "2 règles" {
		t.Fatalf("plurals: %s | %s", fr.N("n", 0), fr.N("n", 2))
	}
	if got := New("en-GB").T("a", "name", "x"); got != "Hello x!" {
		t.Fatal(got)
	}
	if got := New("zz").T("a", "name", "x"); got != "Hello x" {
		t.Fatalf("unknown locale uses en-US: %s", got)
	}
}

var placeholder = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)

func placeholders(s string) string {
	p := placeholder.FindAllString(s, -1)
	sort.Strings(p)
	return strings.Join(p, ",")
}

// isPlural reports whether key is one form of a plural group: it ends in a CLDR category and
// the en-US source has a ".one" form for that base. ("report.sev.other" is just the word
// "Other", not a plural.)
func isPlural(src map[string]string, key string) (base, suffix string, ok bool) {
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", "", false
	}
	switch key[i+1:] {
	case "zero", "one", "two", "few", "many", "other":
		if _, has := src[key[:i]+".one"]; has {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// Every translation must have a key that exists in en-US, with the same placeholders, and
// plural keys must use only the categories the language has.
func TestCatalogsConsistent(t *testing.T) {
	src := catalog(Default)
	for _, tag := range Tags {
		cat := catalog(tag)
		cats := map[string]bool{}
		for _, c := range PluralCategories(tag) {
			cats[c] = true
		}
		groups := map[string]bool{}
		for k, v := range cat {
			base, suffix, plural := isPlural(src, k)
			if plural {
				groups[base] = true
				if tag != Default && !cats[suffix] {
					t.Errorf("%s: plural key %q uses category %q this language does not have", tag, k, suffix)
				}
			} else if _, ok := src[k]; !ok {
				t.Errorf("%s: key %q has no en-US source", tag, k)
			}
			ref, ok := src[k]
			if !ok && plural {
				ref = src[base+".other"]
			}
			if ref != "" && placeholders(ref) != placeholders(v) {
				t.Errorf("%s: %q placeholders %q differ from en-US %q", tag, k, placeholders(v), placeholders(ref))
			}
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: %q is empty", tag, k)
			}
		}
		if tag == Default {
			continue
		}
		// every plural group a locale defines must be complete for the language
		for g := range groups {
			for c := range cats {
				if _, ok := cat[g+"."+c]; !ok {
					t.Errorf("%s: plural group %q is missing the %q form", tag, g, c)
				}
			}
		}
	}
}

func TestBetaLabels(t *testing.T) {
	if IsBeta("en-US") || IsBeta("en-GB") || !IsBeta("vi") || !IsBeta("sr-Cyrl") {
		t.Fatal("only the English source locales are reviewed")
	}
	if Label("en-GB") != "English (UK)" || Label("vi") != "Tiếng Việt · Beta" {
		t.Fatalf("labels: %q %q", Label("en-GB"), Label("vi"))
	}
	for _, tag := range Tags {
		if !strings.HasPrefix(Label(tag), Names[tag]) {
			t.Errorf("%s label must start with its endonym", tag)
		}
	}
}
