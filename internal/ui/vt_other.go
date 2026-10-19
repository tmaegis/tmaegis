// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build !windows

package ui

import "os"

// enableVT is a no-op outside Windows: terminals interpret ANSI sequences natively.
func enableVT(*os.File) bool { return true }
