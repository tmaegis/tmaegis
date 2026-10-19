// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProgressPlainIsPeriodic(t *testing.T) {
	var b bytes.Buffer
	pr := NewProgress(New(&b, Plain), 100, "Auditing", func(done, total, pct int) string {
		return "[*] Progress: " + itoa(done) + "/" + itoa(total)
	})
	now := time.Unix(0, 0)
	pr.now = func() time.Time { return now }
	pr.lastLine = now
	for i := 0; i < 100; i++ {
		if i == 30 || i == 70 {
			now = now.Add(11 * time.Second)
		}
		pr.Inc("R")
	}
	pr.Done()
	if got := b.String(); got != "[*] Progress: 31/100\n[*] Progress: 71/100\n" {
		t.Errorf("plain progress = %q", got)
	}
}

func TestProgressFastRunIsSilent(t *testing.T) {
	var b bytes.Buffer
	pr := NewProgress(New(&b, Plain), 5, "x", func(int, int, int) string { return "line" })
	for i := 0; i < 5; i++ {
		pr.Inc("R")
	}
	pr.Done()
	if b.Len() != 0 {
		t.Errorf("short run printed %q", b.String())
	}
}

func TestProgressLiveLine(t *testing.T) {
	for _, w := range []int{40, 80, 120} {
		for _, uni := range []bool{false, true} {
			var b syncBuf
			c := Caps{TTY: true, Fancy: true, Live: true, Unicode: uni, Width: w}
			pr := NewProgress(New(&b, c), 189, "Auditing", nil)
			var wg sync.WaitGroup
			for i := 0; i < 189; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); pr.Inc("TMA-UBUNTU2404-7.2.9-規則") }()
			}
			wg.Wait()
			line := pr.Render()
			pr.Done()
			want := min(w, MaxWidth) - 1
			if Width(line) != want {
				t.Errorf("w=%d: live line is %d cells: %q", w, Width(line), line)
			}
			if !strings.Contains(line, "189/189") || !strings.Contains(line, "100%") {
				t.Errorf("w=%d: %q", w, line)
			}
			out := b.String()
			if strings.Contains(out, "\n") {
				t.Errorf("live progress must not print newlines: %q", out)
			}
			if !strings.HasSuffix(out, "\r"+strings.Repeat(" ", want)+"\r") {
				t.Errorf("w=%d: line not cleared at the end", w)
			}
		}
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}
