package app

import (
	"os"
	"path/filepath"
	"testing"

	"yups/assets"
	"yups/internal/config"
)

func TestInstallAssets(t *testing.T) {
	tempHome := t.TempDir()

	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
	}

	if err := InstallAssets(env, tempHome); err != nil {
		t.Fatalf("InstallAssets failed: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(config.DataDir(tempHome), "cities.txt"),
		filepath.Join(config.DataDir(tempHome), "whitelist_commands.txt"),
		filepath.Join(config.DataDir(tempHome), "whitelist_wrappers.txt"),
		filepath.Join(config.DataDir(tempHome), "whitelist_conditional_commands.txt"),
		filepath.Join(config.DataDir(tempHome), "theme.toml"),
		filepath.Join(config.PromptsDir(tempHome), "system_prompt.txt"),
		filepath.Join(config.PromptsDir(tempHome), "help.txt"),
	}

	for _, p := range expectedFiles {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("Expected installed asset %s, got error: %v", p, err)
		}
	}
}

func TestEnsureAssetsUpdated(t *testing.T) {
	tempHome := t.TempDir()

	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
		PathExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}

	// First ensure writes all missing assets
	if err := EnsureAssetsUpdated(env, tempHome); err != nil {
		t.Fatalf("EnsureAssetsUpdated failed: %v", err)
	}

	themePath := filepath.Join(config.DataDir(tempHome), "theme.toml")
	if _, err := os.Stat(themePath); err != nil {
		t.Fatalf("theme.toml not created by EnsureAssetsUpdated: %v", err)
	}

	// Override home in assets package to test loading from disk
	assets.SetOverrideHome(tempHome)
	defer assets.ResetOverrideHome()

	// Modify on-disk theme to verify dynamic loading
	customTheme := "error = \"\\x1b[31m\"\nwarning = \"\\x1b[33m\"\n"
	if err := os.WriteFile(themePath, []byte(customTheme), 0o644); err != nil {
		t.Fatalf("Failed writing custom theme: %v", err)
	}

	if got := assets.GetThemeData(); got != customTheme {
		t.Errorf("GetThemeData() = %q, want %q", got, customTheme)
	}
}
