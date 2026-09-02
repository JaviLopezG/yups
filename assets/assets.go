// Package assets provides embedded static resources (shell scripts, python tools, data tables, prompts).
package assets

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// InspectSessionPy contains the contents of inspect-session.py.
//
//go:embed inspect-session.py
var InspectSessionPy string

// ShellYupsBash contains the contents of assets/shell/yups.bash.
//
//go:embed shell/yups.bash
var ShellYupsBash string

// ShellEnvBash contains the contents of assets/shell/env.bash.
//
//go:embed shell/env.bash
var ShellEnvBash string

// ShellCompletionBash contains the contents of assets/shell/completion.bash.
//
//go:embed shell/completion.bash
var ShellCompletionBash string

// ShellKeybindingBash contains the template contents of assets/shell/keybinding.bash.
//
//go:embed shell/keybinding.bash
var ShellKeybindingBash string

// SessionNamesData contains the line-separated list of session slug names.
//
//go:embed data/session-names.txt
var SessionNamesData string

// CitiesData is an alias to SessionNamesData for backwards compatibility.
var CitiesData = SessionNamesData

// WhitelistCommandsData contains the line-separated list of whitelisted inspection commands.
//
//go:embed data/whitelist_commands.txt
var WhitelistCommandsData string

// WhitelistWrappersData contains the line-separated list of safe command wrappers.
//
//go:embed data/whitelist_wrappers.txt
var WhitelistWrappersData string

// WhitelistConditionalCommandsData contains the line-separated list of commands requiring conditional flag inspection.
//
//go:embed data/whitelist_conditional_commands.txt
var WhitelistConditionalCommandsData string

// ThemeData contains the semantic theme TOML definition.
//
//go:embed data/theme.toml
var ThemeData string

// SystemPromptTemplate contains the LLM system prompt template.
//
//go:embed prompts/system_prompt.txt
var SystemPromptTemplate string

// HelpText contains the user-facing CLI help text.
//
//go:embed prompts/help.txt
var HelpText string

var (
	overrideHome string
	overrideMu   sync.RWMutex
)

// SetOverrideHome allows overriding the home directory for asset lookups (useful for tests).
func SetOverrideHome(home string) {
	overrideMu.Lock()
	defer overrideMu.Unlock()
	overrideHome = home
}

// ResetOverrideHome clears any home directory override.
func ResetOverrideHome() {
	overrideMu.Lock()
	defer overrideMu.Unlock()
	overrideHome = ""
}

func readAsset(relPath, embedded string) string {
	overrideMu.RLock()
	home := overrideHome
	overrideMu.RUnlock()

	if home == "" {
		if h, err := os.UserHomeDir(); err == nil && h != "" {
			home = h
		}
	}

	if home != "" {
		diskPath := filepath.Join(home, ".yups", relPath)
		if data, err := os.ReadFile(diskPath); err == nil && len(bytes.TrimSpace(data)) > 0 {
			return string(data)
		}
	}
	return embedded
}

// GetSessionNames returns the slice of names for session slugs.
func GetSessionNames() []string {
	data := readAsset("data/session-names.txt", SessionNamesData)
	var list []string
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			list = append(list, trimmed)
		}
	}
	return list
}

// GetSessionCities is an alias to GetSessionNames for backwards compatibility.
func GetSessionCities() []string {
	return GetSessionNames()
}

// GetWhitelistedCommands returns the map of unconditionally safe read-only inspection commands.
func GetWhitelistedCommands() map[string]bool {
	data := readAsset("data/whitelist_commands.txt", WhitelistCommandsData)
	m := make(map[string]bool)
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			m[trimmed] = true
		}
	}
	return m
}

// GetWhitelistedWrappers returns the map of safe command wrappers.
func GetWhitelistedWrappers() map[string]bool {
	data := readAsset("data/whitelist_wrappers.txt", WhitelistWrappersData)
	m := make(map[string]bool)
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			m[trimmed] = true
		}
	}
	return m
}

// GetWhitelistedConditionalCommands returns the slice of commands requiring conditional flag inspection.
func GetWhitelistedConditionalCommands() []string {
	data := readAsset("data/whitelist_conditional_commands.txt", WhitelistConditionalCommandsData)
	var list []string
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			list = append(list, trimmed)
		}
	}
	return list
}

// GetThemeData returns the raw semantic theme TOML data.
func GetThemeData() string {
	return readAsset("data/theme.toml", ThemeData)
}

// GetSystemPromptTemplate returns the raw system prompt template.
func GetSystemPromptTemplate() string {
	return readAsset("prompts/system_prompt.txt", SystemPromptTemplate)
}

// GetHelpText returns the CLI help text directly from the embedded binary resource.
func GetHelpText() string {
	return HelpText
}
