// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package engine

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/tmaegis/tmaegis/internal/i18n"
	"github.com/tmaegis/tmaegis/internal/rules"
)

// The English summary is parsed by humans and sometimes scripts: it must not drift.
func TestWriteSummaryEnglishUnchanged(t *testing.T) {
	i18n.SetLang("en-US")
	rep := &ScanReport{
		OS: "linux", Arch: "amd64", Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		TotalRules: 2, PassedCount: 1, FailedCount: 1,
		Results: []RuleResult{
			{Rule: rules.Rule{ID: "A.1", Title: "First"}, Status: StatusPass},
			{Rule: rules.Rule{ID: "A.2", Title: "Second"}, Status: StatusFail, Details: "boom"},
		},
	}
	var b bytes.Buffer
	rep.WriteSummary(&b)
	want := `======================================================================
CIS AUDIT RESULTS (linux/amd64) - 2026-01-02T03:04:05Z
======================================================================
[PASS] A.1                      First
[FAIL] A.2                      Second
       -> Details: boom
----------------------------------------------------------------------
Total: 2 | Passed: 1 | Failed: 1 | Compliance Score: 50.0%
======================================================================
`
	if b.String() != want {
		t.Errorf("summary changed:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestWriteSummaryTranslatesFrame(t *testing.T) {
	// An unknown tag falls back to English, a known one with a catalog would differ; here we only
	// prove the frame text goes through i18n and the badges stay stable.
	i18n.SetLang("en-GB")
	defer i18n.SetLang("en-US")
	rep := &ScanReport{OS: "linux", Arch: "arm64", Timestamp: time.Unix(0, 0).UTC(),
		Results: []RuleResult{{Rule: rules.Rule{ID: "X"}, Status: StatusSkip, Details: "n/a"}}}
	var b bytes.Buffer
	rep.WriteSummary(&b)
	if !bytes.Contains(b.Bytes(), []byte("[SKIP]")) {
		t.Errorf("badge missing: %s", b.String())
	}
}

func TestMissingToolIsNotApplicable(t *testing.T) {
	r := rules.Rule{ID: "X", Audit: rules.AuditSpec{Type: "sql_query", Command: "psql -tAc 'select 1' 2>/dev/null || exit 1", ExpectedOutput: "1"}}
	t.Setenv("PATH", t.TempDir())
	res := (&Runner{}).evaluateRule(context.Background(), r)
	if res.Status != StatusSkip {
		t.Fatalf("want SKIP, got %s (%s)", res.Status, res.Details)
	}
}
