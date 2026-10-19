// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package lerr

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/tmaegis/tmaegis/internal/i18n"
)

func TestWrapKeepsCauseAndMessage(t *testing.T) {
	i18n.SetLang("en-US")
	err := Wrap("cli.rollback.err.failed", fs.ErrNotExist)
	if err.Error() != "rollback failed: file does not exist" {
		t.Errorf("got %q", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("cause not unwrappable")
	}
	if got := New("cli.push.err.noToken").Error(); got != "TMAEGIS_SERVER_TOKEN is not set" {
		t.Errorf("got %q", got)
	}
}
