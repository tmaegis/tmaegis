// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build !windows

package gui

import "os"

// isElevated reports whether the process runs as root.
func isElevated() bool { return os.Geteuid() == 0 }
