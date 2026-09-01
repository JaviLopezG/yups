// Package assets provides embedded static resources (shell scripts, python tools, data tables, prompts).
package assets

import (
	_ "embed"
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

// CitiesData contains the line-separated list of session cities.
//
//go:embed data/cities.txt
var CitiesData string

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
	citiesOnce sync.Once
	citiesList []string

	whitelistCmdsOnce sync.Once
	whitelistCmdsMap  map[string]bool

	whitelistWrappersOnce sync.Once
	whitelistWrappersMap  map[string]bool

	whitelistConditionalOnce sync.Once
	whitelistConditionalList []string
)

// GetSessionCities returns the slice of city names for session slugs.
func GetSessionCities() []string {
	citiesOnce.Do(func() {
		for _, line := range strings.Split(CitiesData, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				citiesList = append(citiesList, trimmed)
			}
		}
	})
	return citiesList
}

// GetWhitelistedCommands returns the map of unconditionally safe read-only inspection commands.
func GetWhitelistedCommands() map[string]bool {
	whitelistCmdsOnce.Do(func() {
		whitelistCmdsMap = make(map[string]bool)
		for _, line := range strings.Split(WhitelistCommandsData, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				whitelistCmdsMap[trimmed] = true
			}
		}
	})
	return whitelistCmdsMap
}

// GetWhitelistedWrappers returns the map of safe command wrappers.
func GetWhitelistedWrappers() map[string]bool {
	whitelistWrappersOnce.Do(func() {
		whitelistWrappersMap = make(map[string]bool)
		for _, line := range strings.Split(WhitelistWrappersData, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				whitelistWrappersMap[trimmed] = true
			}
		}
	})
	return whitelistWrappersMap
}

// GetWhitelistedConditionalCommands returns the slice of commands requiring conditional flag inspection.
func GetWhitelistedConditionalCommands() []string {
	whitelistConditionalOnce.Do(func() {
		for _, line := range strings.Split(WhitelistConditionalCommandsData, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				whitelistConditionalList = append(whitelistConditionalList, trimmed)
			}
		}
	})
	return whitelistConditionalList
}

// GetThemeData returns the raw semantic theme TOML data.
func GetThemeData() string {
	return ThemeData
}

// GetSystemPromptTemplate returns the raw system prompt template.
func GetSystemPromptTemplate() string {
	return SystemPromptTemplate
}

// GetHelpText returns the CLI help text.
func GetHelpText() string {
	return HelpText
}
