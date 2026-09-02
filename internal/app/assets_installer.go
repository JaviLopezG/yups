// Copyright (c) 2026, Javi Lopez
// All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"yups/assets"
	"yups/internal/config"
)

// AssetFile represents a static asset to deploy into ~/.yups/.
type AssetFile struct {
	RelPath string // e.g. "data/cities.txt", "prompts/system_prompt.txt"
	Content string
}

// AllAssetFiles returns the collection of static assets deployed to ~/.yups/.
func AllAssetFiles() []AssetFile {
	return []AssetFile{
		{RelPath: filepath.Join("data", "session-names.txt"), Content: assets.SessionNamesData},
		{RelPath: filepath.Join("data", "whitelist_commands.txt"), Content: assets.WhitelistCommandsData},
		{RelPath: filepath.Join("data", "whitelist_wrappers.txt"), Content: assets.WhitelistWrappersData},
		{RelPath: filepath.Join("data", "whitelist_conditional_commands.txt"), Content: assets.WhitelistConditionalCommandsData},
		{RelPath: filepath.Join("data", "theme.toml"), Content: assets.ThemeData},
		{RelPath: filepath.Join("prompts", "system_prompt.txt"), Content: assets.SystemPromptTemplate},
		{RelPath: filepath.Join("prompts", "help.txt"), Content: assets.HelpText},
	}
}

// InstallAssets copies all data and prompt assets to ~/.yups/data/ and ~/.yups/prompts/.
func InstallAssets(env *Env, home string) error {
	for _, a := range AllAssetFiles() {
		targetPath := filepath.Join(config.Dir(home), a.RelPath)
		_ = os.MkdirAll(filepath.Dir(targetPath), 0o755)
		if env != nil && env.WriteFile != nil {
			if err := env.WriteFile(targetPath, []byte(a.Content), 0o644); err != nil {
				return fmt.Errorf("writing asset %s: %w", targetPath, err)
			}
		} else {
			if err := os.WriteFile(targetPath, []byte(a.Content), 0o644); err != nil {
				return fmt.Errorf("writing asset %s: %w", targetPath, err)
			}
		}
	}
	return nil
}

// EnsureAssetsUpdated updates all static data, prompt, and shell script assets under ~/.yups/.
func EnsureAssetsUpdated(env *Env, home string) error {
	for _, a := range AllAssetFiles() {
		targetPath := filepath.Join(config.Dir(home), a.RelPath)
		_ = os.MkdirAll(filepath.Dir(targetPath), 0o755)
		if env != nil && env.WriteFile != nil {
			_ = env.WriteFile(targetPath, []byte(a.Content), 0o644)
		} else {
			_ = os.WriteFile(targetPath, []byte(a.Content), 0o644)
		}
	}

	// Update shell scripts
	shellDir := config.ShellDir(home)
	_ = os.MkdirAll(shellDir, 0o755)
	writeShell := func(rel, content string) {
		p := filepath.Join(shellDir, rel)
		if env != nil && env.WriteFile != nil {
			_ = env.WriteFile(p, []byte(content), 0o644)
		} else {
			_ = os.WriteFile(p, []byte(content), 0o644)
		}
	}
	writeShell("yups.bash", assets.ShellYupsBash)
	writeShell("env.bash", assets.ShellEnvBash)
	writeShell("completion.bash", assets.ShellCompletionBash)

	// Update keybinding.bash preserving existing active binding if set in state
	keySeq := ""
	if env != nil && env.LoadUpdateState != nil {
		if st, err := env.LoadUpdateState(config.StatePath(home)); err == nil && st.Keybinding != "" {
			keySeq = KeyNameToSequence(st.Keybinding)
		}
	} else {
		if st, err := osLoadUpdateState(config.StatePath(home)); err == nil && st.Keybinding != "" {
			keySeq = KeyNameToSequence(st.Keybinding)
		}
	}
	writeShell("keybinding.bash", GenerateKeybindingScriptContent(keySeq))

	return nil
}
