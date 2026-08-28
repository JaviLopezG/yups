// Copyright (c) 2026, Javi Lopez
// All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ANSI escape codes for styling
const (
	AnsiReset     = "\x1b[0m"
	AnsiBold      = "\x1b[1m"
	AnsiDim       = "\x1b[2m"
	AnsiUnderline = "\x1b[4m"
	AnsiOrange    = "\x1b[38;5;214m"
	AnsiBlue      = "\x1b[38;5;39m"
	AnsiCyan      = "\x1b[1;36m"
	AnsiGreen     = "\x1b[1;32m"
	AnsiYellow    = "\x1b[1;33m"
	AnsiRed       = "\x1b[1;31m"
	AnsiGray      = "\x1b[90m"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner renders an animated CLI spinner for asynchronous operations.
type Spinner struct {
	w       io.Writer
	msg     string
	isTTY   bool
	color   bool
	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	stopped bool
	start   time.Time
}

// StartSpinner creates and launches an active spinner on the given writer.
func StartSpinner(w io.Writer, msg string, isTTY bool, color bool) *Spinner {
	s := &Spinner{
		w:      w,
		msg:    msg,
		isTTY:  isTTY,
		color:  color,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		start:  time.Now(),
	}

	if isTTY {
		go s.run()
	} else {
		// In non-interactive mode, close doneCh immediately
		close(s.doneCh)
	}

	return s
}

// UpdateMessage changes the status text displayed next to the spinner.
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = msg
}

// Stop terminates the spinner and cleans up the active terminal line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	if s.isTTY {
		close(s.stopCh)
		<-s.doneCh
		// Clear current line
		fmt.Fprint(s.w, "\r\x1b[2K\r")
	}
}

func (s *Spinner) run() {
	defer close(s.doneCh)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	frameIdx := 0
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.msg
			frame := spinnerFrames[frameIdx%len(spinnerFrames)]
			elapsed := time.Since(s.start).Seconds()
			frameIdx++
			s.mu.Unlock()

			if s.color {
				fmt.Fprintf(s.w, "\r\x1b[2K%s%s%s %s %s(%.1fs)%s",
					AnsiCyan, frame, AnsiReset,
					msg,
					AnsiGray, elapsed, AnsiReset)
			} else {
				fmt.Fprintf(s.w, "\r\x1b[2K%s %s (%.1fs)", frame, msg, elapsed)
			}
		}
	}
}

// RenderProgressBar renders an ASCII/Unicode progress bar.
// Example: "[████████████░░░░░░░░]  60.0% (3/5)"
func RenderProgressBar(current, total int, width int, color bool) string {
	if total <= 0 {
		return ""
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	if width < 5 {
		width = 20
	}

	ratio := float64(current) / float64(total)
	pct := ratio * 100.0
	filled := int(float64(width) * ratio)
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	if color {
		return fmt.Sprintf("[%s%s%s%s%s] %5.1f%% (%d/%d)",
			AnsiGreen, strings.Repeat("█", filled), AnsiReset,
			AnsiGray, strings.Repeat("░", empty)+AnsiReset,
			pct, current, total)
	}

	return fmt.Sprintf("[%s] %5.1f%% (%d/%d)", bar, pct, current, total)
}

// FormatBytes converts raw byte count into human-readable units.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
