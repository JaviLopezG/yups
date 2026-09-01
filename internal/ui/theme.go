// Copyright (c) 2026, Javi Lopez
// All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package ui

import (
	"strings"

	"github.com/BurntSushi/toml"
	"yups/assets"
)

// Theme represents the semantic terminal styling and color palette.
type Theme struct {
	Error     string `toml:"error"`
	Warning   string `toml:"warning"`
	Success   string `toml:"success"`
	Info      string `toml:"info"`
	Important string `toml:"important"`
	Prompt    string `toml:"prompt"`
	Muted     string `toml:"muted"`
	Bold      string `toml:"bold"`
	Dim       string `toml:"dim"`
	Underline string `toml:"underline"`
	Reset     string `toml:"reset"`
}

// DefaultTheme returns the standard semantic ANSI palette.
func DefaultTheme() Theme {
	return Theme{
		Error:     "\x1b[1;31m",
		Warning:   "\x1b[1;33m",
		Success:   "\x1b[1;32m",
		Info:      "\x1b[1;36m",
		Important: "\x1b[38;5;39m",
		Prompt:    "\x1b[38;5;214m",
		Muted:     "\x1b[90m",
		Bold:      "\x1b[1m",
		Dim:       "\x1b[2m",
		Underline: "\x1b[4m",
		Reset:     "\x1b[0m",
	}
}

// colorAliases maps simple color and style names to ANSI escape sequences.
var colorAliases = map[string]string{
	"red":        "\x1b[1;31m",
	"green":      "\x1b[1;32m",
	"yellow":     "\x1b[1;33m",
	"blue":       "\x1b[38;5;39m",
	"cyan":       "\x1b[1;36m",
	"orange":     "\x1b[38;5;214m",
	"gray":       "\x1b[90m",
	"grey":       "\x1b[90m",
	"bold":       "\x1b[1m",
	"dim":        "\x1b[2m",
	"underline":  "\x1b[4m",
	"reset":      "\x1b[0m",
}

func resolveANSI(val, fallback string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return fallback
	}
	if alias, ok := colorAliases[strings.ToLower(val)]; ok {
		return alias
	}
	return val
}

// GetTheme returns the loaded semantic palette, falling back to defaults for any missing fields.
func GetTheme() Theme {
	defaults := DefaultTheme()
	var parsed Theme
	if err := toml.Unmarshal([]byte(assets.GetThemeData()), &parsed); err == nil {
		return Theme{
			Error:     resolveANSI(parsed.Error, defaults.Error),
			Warning:   resolveANSI(parsed.Warning, defaults.Warning),
			Success:   resolveANSI(parsed.Success, defaults.Success),
			Info:      resolveANSI(parsed.Info, defaults.Info),
			Important: resolveANSI(parsed.Important, defaults.Important),
			Prompt:    resolveANSI(parsed.Prompt, defaults.Prompt),
			Muted:     resolveANSI(parsed.Muted, defaults.Muted),
			Bold:      resolveANSI(parsed.Bold, defaults.Bold),
			Dim:       resolveANSI(parsed.Dim, defaults.Dim),
			Underline: resolveANSI(parsed.Underline, defaults.Underline),
			Reset:     resolveANSI(parsed.Reset, defaults.Reset),
		}
	}
	return defaults
}
