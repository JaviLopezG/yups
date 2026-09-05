// Copyright (c) 2026, Javi Lopez
// All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package app

import (
	"os"
	"path/filepath"

	"yups/assets"
	"yups/internal/config"
)

// CleanLegacyAssets removes old auto-exported data and prompts directories from ~/.yups/.
func CleanLegacyAssets(env *Env, home string) {
	removePath := func(p string) {
		if env != nil && env.Remove != nil {
			_ = env.Remove(p)
		} else {
			_ = os.Remove(p)
		}
	}
	legacyFiles := []string{
		filepath.Join(config.Dir(home), "data", "session-names.txt"),
		filepath.Join(config.Dir(home), "data", "whitelist_commands.txt"),
		filepath.Join(config.Dir(home), "data", "whitelist_wrappers.txt"),
		filepath.Join(config.Dir(home), "data", "whitelist_conditional_commands.txt"),
		filepath.Join(config.Dir(home), "data", "theme.toml"),
		filepath.Join(config.Dir(home), "prompts", "system_prompt.txt"),
		filepath.Join(config.Dir(home), "prompts", "help.txt"),
	}
	for _, f := range legacyFiles {
		removePath(f)
	}
	// Also attempt to remove data and prompts directories if empty
	removePath(filepath.Join(config.Dir(home), "data"))
	removePath(filepath.Join(config.Dir(home), "prompts"))
}

// InstallAssets writes ~/.yups/README.md and cleans any legacy assets.
func InstallAssets(env *Env, home string) error {
	CleanLegacyAssets(env, home)
	readmePath := filepath.Join(config.Dir(home), "README.md")
	_ = os.MkdirAll(filepath.Dir(readmePath), 0o755)
	if env != nil && env.WriteFile != nil {
		return env.WriteFile(readmePath, []byte(assets.DotYupsReadme), 0o644)
	}
	return os.WriteFile(readmePath, []byte(assets.DotYupsReadme), 0o644)
}

// EnsureAssetsUpdated updates ~/.yups/README.md, shell scripts, and cleans legacy assets.
func EnsureAssetsUpdated(env *Env, home string) error {
	CleanLegacyAssets(env, home)
	readmePath := filepath.Join(config.Dir(home), "README.md")
	_ = os.MkdirAll(filepath.Dir(readmePath), 0o755)
	if env != nil && env.WriteFile != nil {
		_ = env.WriteFile(readmePath, []byte(assets.DotYupsReadme), 0o644)
	} else {
		_ = os.WriteFile(readmePath, []byte(assets.DotYupsReadme), 0o644)
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
