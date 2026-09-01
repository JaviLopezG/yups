package app

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "abc", 3},
		{"same", "same", 0},
		{"--help", "--help", 0},
		{"--hlp", "--help", 1},
		{"-help", "--help", 1},
		{"--instll-yups", "--install-yups", 1},
		{"--install", "--install-yups", 5},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		got := LevenshteinDistance(tt.s1, tt.s2)
		if got != tt.want {
			t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
		}
	}
}

func TestFindFlagsByLetter(t *testing.T) {
	t.Run("single-match-lowercase", func(t *testing.T) {
		matches := FindFlagsByLetter("h", KnownFlags)
		if len(matches) != 1 || matches[0] != "--help" {
			t.Errorf("FindFlagsByLetter(h) = %v, want [--help]", matches)
		}
	})

	t.Run("single-match-uppercase", func(t *testing.T) {
		matches := FindFlagsByLetter("V", KnownFlags)
		if len(matches) != 1 || matches[0] != "--version" {
			t.Errorf("FindFlagsByLetter(V) = %v, want [--version]", matches)
		}
	})

	t.Run("multiple-matches", func(t *testing.T) {
		matches := FindFlagsByLetter("u", KnownFlags)
		if len(matches) != 2 || matches[0] != "--uninstall-yups" || matches[1] != "--update-yups" {
			t.Errorf("FindFlagsByLetter(u) = %v, want [--uninstall-yups, --update-yups]", matches)
		}
	})

	t.Run("single-match-no-limits", func(t *testing.T) {
		matches := FindFlagsByLetter("n", KnownFlags)
		if len(matches) != 1 || matches[0] != "--no-limits" {
			t.Errorf("FindFlagsByLetter(n) = %v, want [--no-limits]", matches)
		}
	})

	t.Run("zero-matches", func(t *testing.T) {
		matches := FindFlagsByLetter("x", KnownFlags)
		if len(matches) != 0 {
			t.Errorf("FindFlagsByLetter(x) = %v, want []", matches)
		}
	})
}

func TestFindSimilarFlags(t *testing.T) {
	t.Run("prefix-matching", func(t *testing.T) {
		matches := FindSimilarFlags("--install", KnownFlags)
		if len(matches) == 0 || matches[0] != "--install-yups" {
			t.Errorf("FindSimilarFlags(--install) = %v, want [--install-yups]", matches)
		}

		matchesNoLimits := FindSimilarFlags("--no-limit", KnownFlags)
		if len(matchesNoLimits) == 0 || matchesNoLimits[0] != "--no-limits" {
			t.Errorf("FindSimilarFlags(--no-limit) = %v, want [--no-limits]", matchesNoLimits)
		}
	})

	t.Run("prefix-matching-multiple", func(t *testing.T) {
		matches := FindSimilarFlags("--u", KnownFlags)
		if len(matches) < 2 {
			t.Fatalf("FindSimilarFlags(--u) returned %d matches, want at least 2: %v", len(matches), matches)
		}
		hasUninstall := false
		hasUpdate := false
		for _, m := range matches {
			if m == "--uninstall-yups" {
				hasUninstall = true
			}
			if m == "--update-yups" {
				hasUpdate = true
			}
		}
		if !hasUninstall || !hasUpdate {
			t.Errorf("FindSimilarFlags(--u) missing expected prefixes, got: %v", matches)
		}
	})

	t.Run("fuzzy-matching-distance-under-2", func(t *testing.T) {
		matches := FindSimilarFlags("--instll-yups", KnownFlags)
		if len(matches) == 0 || matches[0] != "--install-yups" {
			t.Errorf("FindSimilarFlags(--instll-yups) = %v, want [--install-yups]", matches)
		}

		matchesHelp := FindSimilarFlags("--hlp", KnownFlags)
		if len(matchesHelp) == 0 || matchesHelp[0] != "--help" {
			t.Errorf("FindSimilarFlags(--hlp) = %v, want [--help]", matchesHelp)
		}

		matchesModel := FindSimilarFlags("--modle", KnownFlags)
		if len(matchesModel) == 0 || matchesModel[0] != "--model" {
			t.Errorf("FindSimilarFlags(--modle) = %v, want [--model]", matchesModel)
		}

		matchesNoLimits := FindSimilarFlags("--nolimits", KnownFlags)
		if len(matchesNoLimits) == 0 || matchesNoLimits[0] != "--no-limits" {
			t.Errorf("FindSimilarFlags(--nolimits) = %v, want [--no-limits]", matchesNoLimits)
		}
	})

	t.Run("no-matches-for-distant-strings", func(t *testing.T) {
		matches := FindSimilarFlags("--completely-unrelated-long-flag", KnownFlags)
		if len(matches) != 0 {
			t.Errorf("FindSimilarFlags(unrelated) = %v, want []", matches)
		}
	})
}

func TestHandleUnknownFlag(t *testing.T) {
	t.Run("zero-matches-prints-help", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--completely-nonexistent-flag"}, &stdout, &stderr)

		if code != ExitUsage {
			t.Errorf("expected exit code %d, got %d", ExitUsage, code)
		}
		if !strings.Contains(stderr.String(), `unknown option "--completely-nonexistent-flag"`) {
			t.Errorf("stderr missing unknown option message:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("stderr missing help text:\n%s", stderr.String())
		}
	})

	t.Run("multiple-matches-suggests-list", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--u"}, &stdout, &stderr)

		if code != ExitUsage {
			t.Errorf("expected exit code %d, got %d", ExitUsage, code)
		}
		if !strings.Contains(stderr.String(), "Did you mean one of these?") {
			t.Errorf("stderr missing 'Did you mean one of these?' header:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "--uninstall-yups") || !strings.Contains(stderr.String(), "--update-yups") {
			t.Errorf("stderr missing candidate flags:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "Run 'yups --help' for a list of available options.") {
			t.Errorf("stderr missing help suggestion:\n%s", stderr.String())
		}
	})

	t.Run("short-flag-single-match-suggests-replacement", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskPrompt = func(prompt, defaultValue string) string {
			return "yes"
		}

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"-h"}, &stdout, &stderr)

		if code != ExitOK {
			t.Errorf("expected exit code %d, got %d", ExitOK, code)
		}
		if !strings.Contains(stdout.String(), `yups: yups does not use short flags ("-h"). Did you mean "--help"?`) {
			t.Errorf("stdout missing short flag message:\n%s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "Suggested command:\n  yups --help") {
			t.Errorf("stdout missing suggested command:\n%s", stdout.String())
		}
	})

	t.Run("short-flag-multiple-matches-lists-candidates", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"-u"}, &stdout, &stderr)

		if code != ExitUsage {
			t.Errorf("expected exit code %d, got %d", ExitUsage, code)
		}
		if !strings.Contains(stderr.String(), `yups: yups does not use short flags ("-u")`) {
			t.Errorf("stderr missing short flag notice:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "--uninstall-yups") || !strings.Contains(stderr.String(), "--update-yups") {
			t.Errorf("stderr missing candidate flags:\n%s", stderr.String())
		}
	})

	t.Run("short-flag-zero-matches-prints-help", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"-x"}, &stdout, &stderr)

		if code != ExitUsage {
			t.Errorf("expected exit code %d, got %d", ExitUsage, code)
		}
		if !strings.Contains(stderr.String(), `yups: yups does not use short flags ("-x")`) {
			t.Errorf("stderr missing short flag notice:\n%s", stderr.String())
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("stderr missing help text:\n%s", stderr.String())
		}
	})

	t.Run("single-match-accept-yes-proceeds", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskPrompt = func(prompt, defaultValue string) string {
			return "yes"
		}

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--versio"}, &stdout, &stderr)

		if code != ExitOK {
			t.Errorf("expected exit code %d, got %d", ExitOK, code)
		}
		if !strings.Contains(stdout.String(), "Suggested command:\n  yups --version") {
			t.Errorf("stdout missing suggested command:\n%s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "yups dev") {
			t.Errorf("stdout missing executed version output:\n%s", stdout.String())
		}
	})

	t.Run("single-match-with-equal-preserves-value", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskPrompt = func(prompt, defaultValue string) string {
			return "no"
		}

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--modl=qwen2.5-coder"}, &stdout, &stderr)

		if code != ExitOK {
			t.Errorf("expected exit code %d, got %d", ExitOK, code)
		}
		if !strings.Contains(stdout.String(), "Did you mean \"--model=qwen2.5-coder\"?") {
			t.Errorf("stdout missing matched option with value:\n%s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "Suggested command:\n  yups --model=qwen2.5-coder") {
			t.Errorf("stdout missing suggested command:\n%s", stdout.String())
		}
	})

	t.Run("single-match-decline-no-exits", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskPrompt = func(prompt, defaultValue string) string {
			return "no"
		}

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--versio"}, &stdout, &stderr)

		if code != ExitOK {
			t.Errorf("expected exit code %d, got %d", ExitOK, code)
		}
		if !strings.Contains(stdout.String(), "Usage:") || !strings.Contains(stdout.String(), "--install-yups") {
			t.Errorf("stdout missing full help text:\n%s", stdout.String())
		}
		if strings.Contains(stdout.String(), "yups dev") {
			t.Errorf("command should not have executed when declined")
		}
	})

	t.Run("single-match-edit-runs-edited-command", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		var executedCmd string
		env.ExecShell = func(command string, stdout, stderr io.Writer) int {
			executedCmd = command
			return 0
		}
		env.AskPrompt = func(prompt, defaultValue string) string {
			return "edit"
		}
		env.AskEditPrompt = func(prompt, initialValue string) string {
			return "yups --help"
		}

		var stdout, stderr bytes.Buffer
		code := Dispatch(env, []string{"--versio"}, &stdout, &stderr)

		if code != ExitOK {
			t.Errorf("expected exit code %d, got %d", ExitOK, code)
		}
		if executedCmd != "yups --help" {
			t.Errorf("expected executed command 'yups --help', got %q", executedCmd)
		}
		if !strings.Contains(stdout.String(), "Executing: yups --help") {
			t.Errorf("stdout missing executing notice:\n%s", stdout.String())
		}
	})
}
