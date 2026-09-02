package ui

import (
	"strings"
	"testing"
)

func TestGetTheme(t *testing.T) {
	theme := GetTheme()
	if theme.Error == "" || !strings.Contains(theme.Error, "\x1b[") {
		t.Errorf("theme.Error = %q, want ANSI code", theme.Error)
	}
	if theme.Success == "" || !strings.Contains(theme.Success, "\x1b[") {
		t.Errorf("theme.Success = %q, want ANSI code", theme.Success)
	}
	if theme.Prompt == "" || !strings.Contains(theme.Prompt, "\x1b[") {
		t.Errorf("theme.Prompt = %q, want ANSI code", theme.Prompt)
	}
	if theme.Important == "" || !strings.Contains(theme.Important, "\x1b[") {
		t.Errorf("theme.Important = %q, want ANSI code", theme.Important)
	}
	if theme.Info == "" || !strings.Contains(theme.Info, "\x1b[") {
		t.Errorf("theme.Info = %q, want ANSI code", theme.Info)
	}
	if theme.Warning == "" || !strings.Contains(theme.Warning, "\x1b[") {
		t.Errorf("theme.Warning = %q, want ANSI code", theme.Warning)
	}
	if theme.Muted == "" || !strings.Contains(theme.Muted, "\x1b[") {
		t.Errorf("theme.Muted = %q, want ANSI code", theme.Muted)
	}
}

func TestResolveANSI(t *testing.T) {
	if got := resolveANSI("red", "fallback"); got != "\x1b[1;31m" {
		t.Errorf("resolveANSI(red) = %q, want \\x1b[1;31m", got)
	}
	if got := resolveANSI("\x1b[38;5;123m", "fallback"); got != "\x1b[38;5;123m" {
		t.Errorf("resolveANSI(custom) = %q, want custom", got)
	}
	if got := resolveANSI("", "fallback"); got != "fallback" {
		t.Errorf("resolveANSI(empty) = %q, want fallback", got)
	}
}
