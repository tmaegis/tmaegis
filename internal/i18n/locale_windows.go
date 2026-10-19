// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

// systemLocale returns the user's Windows display locale, e.g. "vi-VN" or "zh-Hans-CN".
func systemLocale() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	buf := make([]uint16, 85)
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
