// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

//go:build windows

package cmd

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

// initConsole makes Windows consoles render UTF-8 (non-Latin translations) and ANSI sequences.
// Failures are ignored: output then falls back to the console's code page.
func initConsole() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	k32.NewProc("SetConsoleOutputCP").Call(65001)
	k32.NewProc("SetConsoleCP").Call(65001)
	getMode := k32.NewProc("GetConsoleMode")
	setMode := k32.NewProc("SetConsoleMode")
	for _, f := range []*os.File{os.Stdout, os.Stderr} {
		var mode uint32
		h := f.Fd()
		if r, _, _ := getMode.Call(h, uintptr(unsafe.Pointer(&mode))); r == 0 {
			continue // not a console (redirected)
		}
		if mode&enableVirtualTerminalProcessing == 0 {
			setMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
		}
	}
}
