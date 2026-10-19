// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Package cloud holds agentless, read-only CIS checks for AWS, Azure and GCP built on the
// official SDKs. Each check returns per-resource findings and, for IaC-managed estates,
// the Terraform needed to fix a failing resource, so changes go through review rather
// than being applied directly (avoiding Terraform state drift).
package cloud

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Finding is the result of evaluating one cloud resource.
type Finding struct {
	Resource  string `json:"resource"`
	Compliant bool   `json:"compliant"`
	Detail    string `json:"detail,omitempty"`
}

// Check is a registered, read-only cloud control.
type Check struct {
	ID       string // e.g. aws.s3.account_public_access_block
	Provider string // aws, azure, gcp
	Title    string
	Run      func(ctx context.Context) ([]Finding, error)
	// Terraform renders the HCL that makes a non-compliant resource compliant.
	Terraform func(f Finding) string
}

var registry = map[string]*Check{}

func register(c *Check) {
	if _, dup := registry[c.ID]; dup {
		panic("duplicate cloud check " + c.ID)
	}
	registry[c.ID] = c
}

// Get returns the check registered under id.
func Get(id string) (*Check, bool) {
	c, ok := registry[id]
	return c, ok
}

// IDs lists all registered checks, sorted.
func IDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Summarize reports whether every finding is compliant, with a human-readable explanation.
func Summarize(findings []Finding) (bool, string) {
	if len(findings) == 0 {
		return false, "no resources evaluated"
	}
	var bad []string
	for _, f := range findings {
		if !f.Compliant {
			s := f.Resource
			if f.Detail != "" {
				s += " (" + f.Detail + ")"
			}
			bad = append(bad, s)
		}
	}
	if len(bad) == 0 {
		return true, ""
	}
	return false, fmt.Sprintf("%d non-compliant: %s", len(bad), strings.Join(bad, "; "))
}

// tfName turns a cloud resource name into a valid Terraform identifier.
func tfName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "r_" + out
	}
	return out
}
