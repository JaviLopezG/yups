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
		{RelPath: filepath.Join("data", "cities.txt"), Content: assets.CitiesData},
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

// EnsureAssetsUpdated verifies all assets exist under ~/.yups/ and writes any missing assets.
func EnsureAssetsUpdated(env *Env, home string) error {
	for _, a := range AllAssetFiles() {
		targetPath := filepath.Join(config.Dir(home), a.RelPath)
		exists := false
		if env != nil && env.PathExists != nil {
			exists = env.PathExists(targetPath)
		} else {
			_, err := os.Stat(targetPath)
			exists = err == nil
		}
		if !exists {
			_ = os.MkdirAll(filepath.Dir(targetPath), 0o755)
			if env != nil && env.WriteFile != nil {
				_ = env.WriteFile(targetPath, []byte(a.Content), 0o644)
			} else {
				_ = os.WriteFile(targetPath, []byte(a.Content), 0o644)
			}
		}
	}
	return nil
}
