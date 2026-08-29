// Copyright (c) 2026, Javi Lopez
// All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		width    int
		color    bool
		wantSub  string
	}{
		{
			name:    "zero progress",
			current: 0,
			total:   10,
			width:   10,
			color:   false,
			wantSub: "[░░░░░░░░░░]   0.0% (0/10)",
		},
		{
			name:    "half progress",
			current: 5,
			total:   10,
			width:   10,
			color:   false,
			wantSub: "[█████░░░░░]  50.0% (5/10)",
		},
		{
			name:    "full progress",
			current: 10,
			total:   10,
			width:   10,
			color:   false,
			wantSub: "[██████████] 100.0% (10/10)",
		},
		{
			name:    "colored output contains ANSI escapes",
			current: 3,
			total:   6,
			width:   10,
			color:   true,
			wantSub: AnsiGreen,
		},
		{
			name:    "invalid total returns empty",
			current: 1,
			total:   0,
			width:   10,
			color:   false,
			wantSub: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderProgressBar(tc.current, tc.total, tc.width, tc.color)
			if tc.wantSub == "" {
				if got != "" {
					t.Errorf("got %q, want empty string", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Errorf("RenderProgressBar() = %q, want containing %q", got, tc.wantSub)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{524288000, "500.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := FormatBytes(tc.bytes)
			if got != tc.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestSpinnerNonTTY(t *testing.T) {
	var buf bytes.Buffer
	s := StartSpinner(&buf, "Loading test", false, false)
	s.UpdateMessage("Updating test")
	s.Stop()

	// In non-TTY mode, spinner stays silent to avoid polluting non-interactive streams
	if buf.Len() > 0 {
		t.Errorf("non-TTY spinner output = %q, want empty", buf.String())
	}
}

func TestSpinnerTTY(t *testing.T) {
	var buf bytes.Buffer
	s := StartSpinner(&buf, "Working", true, false)
	time.Sleep(150 * time.Millisecond)
	s.UpdateMessage("Still working")
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	out := buf.String()
	if !strings.Contains(out, "Working") && !strings.Contains(out, "Still working") {
		t.Errorf("TTY spinner output missing message, got: %q", out)
	}
}
