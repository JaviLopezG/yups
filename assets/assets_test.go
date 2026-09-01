package assets

import (
	"strings"
	"testing"
)

func TestGetSessionCities(t *testing.T) {
	cities := GetSessionCities()
	if len(cities) < 50 {
		t.Fatalf("GetSessionCities() returned %d cities, want >= 50", len(cities))
	}
	for _, expected := range []string{"seville", "barcelona", "madrid", "valencia", "cadiz"} {
		found := false
		for _, c := range cities {
			if c == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetSessionCities() missing expected city %q", expected)
		}
	}
}

func TestGetWhitelistedCommands(t *testing.T) {
	cmds := GetWhitelistedCommands()
	if len(cmds) < 30 {
		t.Fatalf("GetWhitelistedCommands() returned %d commands, want >= 30", len(cmds))
	}
	for _, required := range []string{"ls", "grep", "cat", "find", "stat", "pwd", "uname"} {
		if !cmds[required] {
			t.Errorf("GetWhitelistedCommands() missing required command %q", required)
		}
	}
}

func TestGetWhitelistedWrappers(t *testing.T) {
	wrappers := GetWhitelistedWrappers()
	for _, req := range []string{"sudo", "env", "time", "nohup", "nice"} {
		if !wrappers[req] {
			t.Errorf("GetWhitelistedWrappers() missing wrapper %q", req)
		}
	}
}

func TestGetSystemPromptTemplate(t *testing.T) {
	prompt := GetSystemPromptTemplate()
	if !strings.Contains(prompt, "user-input-{{NONCE}}") {
		t.Errorf("SystemPromptTemplate missing user-input-{{NONCE}}")
	}
	if !strings.Contains(prompt, "suggested-command") {
		t.Errorf("SystemPromptTemplate missing suggested-command")
	}
}

func TestGetHelpText(t *testing.T) {
	help := GetHelpText()
	if !strings.Contains(help, "yups - fast, concise, contextual terminal assistant") {
		t.Errorf("HelpText missing introductory description")
	}
	if !strings.Contains(help, "--install-yups") {
		t.Errorf("HelpText missing --install-yups")
	}
}

func TestGetWhitelistedConditionalCommands(t *testing.T) {
	cmds := GetWhitelistedConditionalCommands()
	if len(cmds) < 10 {
		t.Fatalf("GetWhitelistedConditionalCommands() returned %d commands, want >= 10", len(cmds))
	}
	for _, req := range []string{"rsync", "make", "bash", "sh", "apt", "git", "cargo", "npm"} {
		found := false
		for _, c := range cmds {
			if c == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetWhitelistedConditionalCommands() missing %q", req)
		}
	}
}

func TestGetThemeData(t *testing.T) {
	theme := GetThemeData()
	for _, key := range []string{"error", "warning", "success", "info", "important", "prompt", "muted"} {
		if !strings.Contains(theme, key) {
			t.Errorf("ThemeData missing key %q", key)
		}
	}
}
