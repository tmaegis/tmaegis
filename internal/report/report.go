// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package report serialises scan results for CI/CD and SIEM consumers.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// Formats lists every format Write supports (plus "text", handled by the CLI).
var Formats = []string{"json", "jsonl", "sarif", "asff", "csv", "html", "md", "xlsx", "pdf"}

// Write renders the report in the requested format. xlsx and pdf are binary. The language of
// HTML, Markdown, XLSX and PDF reports is Lang (or the detected locale when Lang is empty).
func Write(w io.Writer, format string, rep *engine.ScanReport) error {
	return WriteWith(w, format, rep, "", DefaultInfo)
}

// WriteLang is Write with an explicit report language (an internal/i18n tag such as "vi" or
// "zh-Hans"; empty means the default). Reports that carry translated text (HTML, XLSX, PDF) use
// it; the others ignore it. It is safe to call from several goroutines with different languages.
func WriteLang(w io.Writer, format string, rep *engine.ScanReport, lang string) error {
	return WriteWith(w, format, rep, lang, DefaultInfo)
}

// WriteWith is WriteLang plus scan facts (tool version, rules directory, duration, options) that
// the About tab and the XLSX/PDF summaries show.
func WriteWith(w io.Writer, format string, rep *engine.ScanReport, lang string, info Info) error {
	c := newCtx(lang, info)
	switch strings.ToLower(format) {
	case "json":
		return encode(w, rep)
	case "jsonl":
		return writeJSONL(w, rep)
	case "sarif":
		return encode(w, toSARIF(rep))
	case "asff":
		return encode(w, toASFF(rep))
	case "csv":
		return writeCSV(w, rep)
	case "html":
		return writeHTML(w, rep, c)
	case "md":
		return writeMarkdown(w, rep, c)
	case "xlsx":
		return writeXLSX(w, rep, c)
	case "pdf":
		return writePDF(w, rep, c)
	default:
		return fmt.Errorf("unsupported report format %q (use text, %s)", format, strings.Join(Formats, ", "))
	}
}

func encode(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string  `json:"id"`
	ShortDescription sarifTx `json:"shortDescription"`
}

type sarifTx struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID  string  `json:"ruleId"`
	Level   string  `json:"level"`
	Message sarifTx `json:"message"`
}

func toSARIF(rep *engine.ScanReport) sarifLog {
	run := sarifRun{Tool: sarifTool{Driver: sarifDriver{Name: "TMAegis"}}, Results: []sarifResult{}}
	run.Tool.Driver.Rules = []sarifRule{}
	for _, res := range rep.Results {
		run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, sarifRule{
			ID:               res.Rule.ID,
			ShortDescription: sarifTx{Text: res.Rule.Title},
		})
		if res.Status != engine.StatusFail && res.Status != engine.StatusErr {
			continue
		}
		msg := res.Rule.Title
		if res.Details != "" {
			msg += ": " + res.Details
		}
		run.Results = append(run.Results, sarifResult{
			RuleID:  res.Rule.ID,
			Level:   sarifLevel(res.Rule.Severity),
			Message: sarifTx{Text: msg},
		})
	}
	return sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{run},
	}
}

func sarifLevel(sev string) string {
	switch strings.ToLower(sev) {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "note"
	}
}
