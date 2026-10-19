// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build !windows

package cmd

// initConsole is a no-op outside Windows (terminals are UTF-8 already).
func initConsole() {}
