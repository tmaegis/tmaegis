// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Progress reports a long-running task on stderr. On an interactive terminal it is one
// self-updating line (spinner, bar, count, current item), redrawn at most every Throttle.
// Elsewhere (pipes, CI) it prints a plain line at most every Interval, and nothing at all
// for tasks that finish sooner, so short runs add no noise to logs.
type Progress struct {
	p        *Printer
	total    int
	label    string
	plain    func(done, total, pct int) string
	Throttle time.Duration
	Interval time.Duration
	now      func() time.Time

	mu       sync.Mutex
	done     int
	current  string
	frame    int
	lastDraw time.Time
	lastLine time.Time
	drawn    bool
	stop     chan struct{}
	stopped  sync.WaitGroup
}

// NewProgress starts a progress reporter for total items. label is shown on the live line;
// plain formats the periodic non-interactive line (already localized, with its marker).
func NewProgress(p *Printer, total int, label string, plain func(done, total, pct int) string) *Progress {
	pr := &Progress{p: p, total: total, label: label, plain: plain,
		Throttle: 80 * time.Millisecond, Interval: 10 * time.Second, now: time.Now}
	pr.lastLine = pr.now()
	if p.c.Live {
		pr.stop = make(chan struct{})
		pr.stopped.Add(1)
		go pr.animate()
	}
	return pr
}

func (pr *Progress) animate() {
	defer pr.stopped.Done()
	t := time.NewTicker(120 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-pr.stop:
			return
		case <-t.C:
			pr.mu.Lock()
			pr.frame++
			pr.draw(true)
			pr.mu.Unlock()
		}
	}
}

// Inc marks one more item done; item names what was just finished. Safe for concurrent use.
func (pr *Progress) Inc(item string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.done++
	pr.current = item
	if pr.p.c.Live {
		pr.draw(false)
		return
	}
	if pr.plain != nil && pr.now().Sub(pr.lastLine) >= pr.Interval && pr.done < pr.total {
		pr.lastLine = pr.now()
		pr.p.Println(pr.plain(pr.done, pr.total, pr.pct()))
	}
}

func (pr *Progress) pct() int {
	if pr.total <= 0 {
		return 100
	}
	return pr.done * 100 / pr.total
}

// draw repaints the live line (caller holds mu). Unless forced it is throttled.
func (pr *Progress) draw(force bool) {
	if !force && pr.drawn && pr.now().Sub(pr.lastDraw) < pr.Throttle && pr.done < pr.total {
		return
	}
	pr.lastDraw = pr.now()
	pr.drawn = true
	pr.p.Print("\r" + pr.Render() + "\r")
}

// Render is the live line, exactly Width-1 cells wide so it never wraps and fully
// overwrites the previous one (no erase sequence needed, which dumb consoles lack).
func (pr *Progress) Render() string {
	p := pr.p
	g := p.g
	spin := g.Spinner[pr.frame%len(g.Spinner)]
	w := p.Width() - 1
	barW := 24
	if w < 60 {
		barW = 10
	}
	pct := pr.pct()
	filled := pct * barW / 100
	bar := p.Paint(strings.Repeat(g.Full, filled), fg(Cyan)) + p.Dim(strings.Repeat(g.Empty, barW-filled))
	if !p.c.Unicode {
		bar = "[" + bar + "]"
	}
	count := fmt.Sprintf("%d/%d", pr.done, pr.total)
	line := p.Accent(spin) + " " + p.Bold(pr.label) + "  " + bar + "  " + count + "  " + p.Bold(fmt.Sprintf("%3d%%", pct))
	if rest := w - Width(line) - 2; rest > 6 && pr.current != "" {
		line += "  " + p.Dim(Truncate(pr.current, rest, g.Ellipsis))
	}
	return Fit(line, w, g.Ellipsis)
}

// Done stops the reporter and clears the live line.
func (pr *Progress) Done() {
	if pr.stop != nil {
		close(pr.stop)
		pr.stopped.Wait()
		pr.stop = nil
	}
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.drawn {
		pr.p.Print("\r" + strings.Repeat(" ", pr.p.Width()-1) + "\r")
		pr.drawn = false
	}
}
