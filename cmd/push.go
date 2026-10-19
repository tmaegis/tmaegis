// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/tmaegis/tmaegis/internal/lerr"
	"net/http"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

// pushReport uploads a scan report to a tmaegis-server instance.
func pushReport(ctx context.Context, base, token string, rep *engine.ScanReport) error {
	if token == "" {
		return lerr.New("cli.push.err.noToken")
	}
	body, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/api/reports", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return lerr.New("cli.push.err.status", "status", resp.Status)
	}
	return nil
}
