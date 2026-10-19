// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmaegis/tmaegis/internal/i18n"
)

func enCatalog(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile("../internal/i18n/locales/en-US/cli.json")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func hasKey(cat map[string]string, k string) bool {
	if _, ok := cat[k]; ok {
		return true
	}
	_, one := cat[k+".one"]
	_, other := cat[k+".other"]
	return one && other
}

// Every "cli.*" key written in Go source must exist in the en-US catalog.
func TestAllReferencedKeysExist(t *testing.T) {
	cat := enCatalog(t)
	re := regexp.MustCompile(`"(cli\.[A-Za-z0-9_.-]*[A-Za-z0-9_])"`)
	var files []string
	for _, pat := range []string{"*.go", "../internal/engine/*.go", "../internal/snapshot/*.go", "../internal/lerr/*.go"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	found := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			found++
			if !hasKey(cat, m[1]) {
				t.Errorf("%s references missing key %s", f, m[1])
			}
		}
	}
	if found < 50 {
		t.Errorf("only %d keys found; the scan is probably broken", found)
	}
}

// Every command and flag must have help text in the catalog (so none is left blank).
func TestEveryCommandAndFlagHasCatalogText(t *testing.T) {
	cat := enCatalog(t)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		key := cmdKey(c)
		if _, ok := cat["cli.cmd."+key+".short"]; !ok && c.Name() != "help" && c.Name() != "completion" {
			t.Errorf("missing cli.cmd.%s.short", key)
		}
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" || f.Name == "version" {
				return
			}
			if _, ok := cat["cli.flag."+key+"."+f.Name]; !ok {
				t.Errorf("missing cli.flag.%s.%s", key, f.Name)
			}
		})
		for _, s := range c.Commands() {
			if s.Name() == "completion" || s.Name() == "help" {
				continue
			}
			walk(s)
		}
	}
	walk(rootCmd)
}

func TestEnglishHelpUnchanged(t *testing.T) {
	applyLanguage("en-US")
	if got, want := rootCmd.Short, "TMAegis - AI-Powered Cross-Platform CIS Hardening & Compliance Engine"; got != want {
		t.Errorf("root short = %q", got)
	}
	if !strings.HasPrefix(rootCmd.Long, Banner()) || !strings.Contains(rootCmd.Long, "BYOK/Local AI analysis (Claude, OpenAI, Gemini, Ollama).") {
		t.Errorf("root long wrong: %q", rootCmd.Long)
	}
	if f := scanCmd.Flags().Lookup("format"); !strings.HasPrefix(f.Usage, "Report format: text, ") || !strings.Contains(f.Usage, "json") {
		t.Errorf("format usage = %q", f.Usage)
	}
	if got := scanCmd.Flags().Lookup("level").Usage; got != "CIS Profile Level (1 or 2)" {
		t.Errorf("level usage = %q", got)
	}
	if got := rootCmd.UsageTemplate(); !strings.Contains(got, "Available Commands:") {
		t.Errorf("template changed for English")
	}
	var b bytes.Buffer
	rootCmd.SetOut(&b)
	_ = rootCmd.Usage()
	rootCmd.SetOut(nil)
	if !strings.Contains(b.String(), "--lang string") || !strings.Contains(b.String(), "Display language") {
		t.Errorf("usage lacks --lang:\n%s", b.String())
	}
}

type fakeTr map[string]string

func (f fakeTr) Has(k string) bool { _, ok := f[k]; return ok }
func (f fakeTr) T(k string, a ...any) string {
	s, ok := f[k]
	if !ok {
		return k
	}
	for i := 0; i+1 < len(a); i += 2 {
		s = strings.ReplaceAll(s, "{"+a[i].(string)+"}", toS(a[i+1]))
	}
	return s
}
func toS(v any) string { return fmt.Sprint(v) }

// --help is rendered by cobra before any PreRun hook; localizeCommands is what makes
// `tmaegis --help --lang vi` translated, so prove it with an injected translator.
func TestLocalizeCommandsWithInjectedTranslator(t *testing.T) {
	defer applyLanguage("en-US")
	vi := fakeTr{
		"cli.cmd.root.short":         "VI root",
		"cli.cmd.root.long":          "VI long",
		"cli.cmd.scan.short":         "VI scan",
		"cli.flag.scan.level":        "VI level",
		"cli.flag.root.lang":         "VI lang",
		"cli.flag.help":              "VI help {name}",
		"cli.help.usage":             "Cach dung:",
		"cli.help.availableCommands": "Lenh:",
		"cli.help.flags":             "Co:",
		"cli.help.globalFlags":       "Co chung:",
		"cli.help.moreInfo":          "Dung {command} --help",
	}
	localizeCommands(rootCmd, vi)
	if rootCmd.Short != "VI root" || scanCmd.Short != "VI scan" {
		t.Errorf("short not localised: %q %q", rootCmd.Short, scanCmd.Short)
	}
	if !strings.HasSuffix(rootCmd.Long, "VI long") {
		t.Errorf("long = %q", rootCmd.Long)
	}
	if got := scanCmd.Flags().Lookup("level").Usage; got != "VI level" {
		t.Errorf("flag usage = %q", got)
	}
	if got := scanCmd.Flags().Lookup("help").Usage; got != "VI help scan" {
		t.Errorf("help flag usage = %q", got)
	}
	// Keys the translator lacks keep the previous text.
	if got := scanCmd.Flags().Lookup("target").Usage; !strings.Contains(got, "Target platform") {
		t.Errorf("untranslated flag lost text: %q", got)
	}
	var b bytes.Buffer
	rootCmd.SetOut(&b)
	_ = rootCmd.Usage()
	rootCmd.SetOut(nil)
	scanCmd.SetOut(&b)
	_ = scanCmd.Usage()
	scanCmd.SetOut(nil)
	out := b.String()
	for _, want := range []string{"Cach dung:", "Lenh:", "Co chung:", "VI scan", "VI lang", "Dung tmaegis --help"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage missing %q:\n%s", want, out)
		}
	}
}

func TestResolveLanguagePrecedence(t *testing.T) {
	t.Setenv("TMAEGIS_LANG", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "de_DE.UTF-8")
	if got := resolveLanguage("", ""); got != "de" {
		t.Errorf("OS locale: %s", got)
	}
	if got := resolveLanguage("", "fr"); got != "fr" {
		t.Errorf("config beats OS: %s", got)
	}
	t.Setenv("TMAEGIS_LANG", "ja")
	if got := resolveLanguage("", "fr"); got != "ja" {
		t.Errorf("env beats config: %s", got)
	}
	if got := resolveLanguage("vi", "fr"); got != "vi" {
		t.Errorf("flag wins: %s", got)
	}
}

func TestLanguageFromArgs(t *testing.T) {
	t.Setenv("TMAEGIS_LANG", "")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(cfgPath, []byte("ui:\n  language: ko\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		args []string
		want string
		list bool
	}{
		{[]string{"--help", "--lang", "vi"}, "vi", false},
		{[]string{"scan", "--lang=zh-TW"}, "zh-Hant", false},
		{[]string{"--lang", "list"}, i18n.Default, true},
		{[]string{"-c", cfgPath, "--help"}, "ko", false},
		{[]string{"--config=" + cfgPath, "--lang", "tr"}, "tr", false},
	}
	for _, c := range cases {
		got, list := languageFromArgs(c.args)
		if got != c.want && !(c.list && list) {
			t.Errorf("%v = %s want %s", c.args, got, c.want)
		}
		if list != c.list {
			t.Errorf("%v list = %v", c.args, list)
		}
	}
}

func TestPrintLanguages(t *testing.T) {
	i18n.SetLang("vi")
	defer i18n.SetLang("en-US")
	var b bytes.Buffer
	PrintLanguages(&b)
	out := b.String()
	for _, tag := range i18n.Tags {
		if !strings.Contains(out, tag) || !strings.Contains(out, i18n.Names[tag]) {
			t.Errorf("missing %s", tag)
		}
	}
	if !strings.Contains(out, " * vi ") {
		t.Errorf("active locale not marked:\n%s", out)
	}
}

func TestConfirmWord(t *testing.T) {
	for _, in := range []string{"confirm\n", "  CONFIRM ", "xacnhan\n"} {
		if !isConfirmWord(in, "xacnhan") {
			t.Errorf("%q should confirm", in)
		}
	}
	if isConfirmWord("yes\n", "xacnhan") {
		t.Error("yes must not confirm")
	}
}
