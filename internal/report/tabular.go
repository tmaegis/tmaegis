// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

var tableHeader = []string{"Rule ID", "Title", "Target", "Level", "Severity", "Status", "Details"}

func rows(rep *engine.ScanReport) [][]string {
	out := make([][]string, 0, len(rep.Results))
	for _, r := range rep.Results {
		out = append(out, []string{
			r.Rule.ID, r.Rule.Title, r.Rule.Target, fmt.Sprint(r.Rule.Level),
			r.Rule.Severity, string(r.Status), r.Details,
		})
	}
	return out
}

func score(rep *engine.ScanReport) float64 { return rep.Score() }

// csvSafe neutralises spreadsheet formula injection: a cell starting with = + - @ is
// executed by Excel/Sheets, and rule details can contain attacker-influenced command output.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func writeCSV(w io.Writer, rep *engine.ScanReport) error {
	cw := csv.NewWriter(w)
	cw.Write(tableHeader)
	for _, row := range rows(rep) {
		for i := range row {
			row[i] = csvSafe(row[i])
		}
		cw.Write(row)
	}
	cw.Flush()
	return cw.Error()
}

func writeJSONL(w io.Writer, rep *engine.ScanReport) error {
	enc := json.NewEncoder(w)
	for _, r := range rep.Results {
		if err := enc.Encode(map[string]interface{}{
			"timestamp": rep.Timestamp, "os": rep.OS, "arch": rep.Arch,
			"rule_id": r.Rule.ID, "title": r.Rule.Title, "target": r.Rule.Target,
			"severity": r.Rule.Severity, "status": r.Status, "details": r.Details,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ASFFOptions identifies the AWS account/region used when exporting findings for Security Hub.
var ASFFOptions = struct{ AccountID, Region string }{AccountID: "000000000000", Region: "us-east-1"}

func asffSeverity(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return "CRITICAL"
	case "high":
		return "HIGH"
	case "medium":
		return "MEDIUM"
	case "low":
		return "LOW"
	}
	return "INFORMATIONAL"
}

// toASFF converts results to AWS Security Finding Format (BatchImportFindings input).
func toASFF(rep *engine.ScanReport) []map[string]interface{} {
	now := time.Now().UTC().Format(time.RFC3339)
	productARN := fmt.Sprintf("arn:aws:securityhub:%s:%s:product/%s/default", ASFFOptions.Region, ASFFOptions.AccountID, ASFFOptions.AccountID)
	out := []map[string]interface{}{}
	for _, r := range rep.Results {
		status := "PASSED"
		switch r.Status {
		case engine.StatusFail, engine.StatusErr:
			status = "FAILED"
		case engine.StatusSkip:
			status = "NOT_AVAILABLE"
		}
		desc := r.Rule.Description
		if desc == "" {
			desc = r.Rule.Title
		}
		out = append(out, map[string]interface{}{
			"SchemaVersion": "2018-10-08",
			"Id":            fmt.Sprintf("tmaegis/%s/%s", rep.OS, r.Rule.ID),
			"ProductArn":    productARN,
			"GeneratorId":   "tmaegis/" + r.Rule.ID,
			"AwsAccountId":  ASFFOptions.AccountID,
			"Types":         []string{"Software and Configuration Checks/Industry and Regulatory Standards/CIS"},
			"CreatedAt":     now,
			"UpdatedAt":     now,
			"Severity":      map[string]string{"Label": asffSeverity(r.Rule.Severity)},
			"Title":         r.Rule.Title,
			"Description":   desc,
			"Resources":     []map[string]string{{"Type": "Other", "Id": rep.OS + "/" + r.Rule.Target}},
			"Compliance":    map[string]string{"Status": status},
		})
	}
	return out
}
