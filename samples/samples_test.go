// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package samples

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaegis/tmaegis/internal/engine"
	"github.com/tmaegis/tmaegis/internal/rules"
)

type manifestEntry struct {
	Target       string `json:"target"`
	Compliant    string `json:"compliant"`
	Noncompliant string `json:"noncompliant"`
	Rules        int    `json:"rules"`
	Pass         struct {
		Compliant    int `json:"compliant"`
		Noncompliant int `json:"noncompliant"`
	} `json:"pass"`
}

// passCount audits the device config with the target's level-1 config_regex rules.
func passCount(t *testing.T, ruleList []rules.Rule, file string) int {
	t.Helper()
	run := engine.NewRunnerWithConcurrency(4)
	run.DeviceConfig = file
	rep := run.RunAudit(context.Background(), ruleList)
	return rep.PassedCount
}

func TestSamplesComplianceGap(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []manifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("manifest is empty")
	}
	// Load the network rules once; loading per target would repeat the directory walk.
	all, err := rules.LoadFiltered(filepath.Join("..", "rules", "network"), rules.Filter{MaxLevel: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Run(e.Target, func(t *testing.T) {
			comp, non := filepath.FromSlash(e.Compliant), filepath.FromSlash(e.Noncompliant)
			for _, f := range []string{comp, non} {
				if st, err := os.Stat(f); err != nil || st.IsDir() {
					t.Fatalf("sample file missing: %s (%v)", f, err)
				}
			}
			var dev []rules.Rule
			for _, r := range all {
				if r.Target == e.Target && r.Audit.Type == "config_regex" {
					dev = append(dev, r)
				}
			}
			if len(dev) != e.Rules {
				t.Errorf("manifest says %d device rules, found %d", e.Rules, len(dev))
			}
			if len(dev) == 0 {
				t.Skipf("%s has no automated device rules (manual-review only)", e.Target)
			}
			pc, pn := passCount(t, dev, comp), passCount(t, dev, non)
			if pc <= pn {
				t.Errorf("compliant sample passes %d rules, non-compliant %d: want compliant > non-compliant", pc, pn)
			}
			if pc != e.Pass.Compliant || pn != e.Pass.Noncompliant {
				t.Errorf("manifest pass counts (%d/%d) differ from measured (%d/%d)", e.Pass.Compliant, e.Pass.Noncompliant, pc, pn)
			}
		})
	}
}
