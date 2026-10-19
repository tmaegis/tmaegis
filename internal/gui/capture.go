// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package gui

import (
	"bytes"
	"io"
	"os"
	"sync"
)

var captureMu sync.Mutex

// capture runs fn while os.Stdout is redirected, and returns what was printed. The engine
// and snapshot packages report progress with fmt.Print*, so this is how the GUI shows the
// same output the CLI prints. Captures are serialised because os.Stdout is process-wide.
func capture(fn func() error) (string, error) {
	captureMu.Lock()
	defer captureMu.Unlock()
	r, w, err := os.Pipe()
	if err != nil {
		return "", fn()
	}
	orig := os.Stdout
	os.Stdout = w
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() { _, _ = io.Copy(&buf, r); close(done) }()
	ferr := func() (e error) {
		defer func() {
			os.Stdout = orig
			w.Close()
		}()
		return fn()
	}()
	<-done
	r.Close()
	return buf.String(), ferr
}
