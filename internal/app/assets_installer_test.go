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

	// Pre-create legacy asset files to verify cleanup
	legacyData := filepath.Join(config.DataDir(tempHome), "session-names.txt")
	legacyPrompt := filepath.Join(config.PromptsDir(tempHome), "system_prompt.txt")
	_ = os.MkdirAll(filepath.Dir(legacyData), 0o755)
	_ = os.MkdirAll(filepath.Dir(legacyPrompt), 0o755)
	_ = os.WriteFile(legacyData, []byte("old"), 0o644)
	_ = os.WriteFile(legacyPrompt, []byte("old"), 0o644)

	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
		Remove:      os.Remove,
	}

	if err := InstallAssets(env, tempHome); err != nil {
		t.Fatalf("InstallAssets failed: %v", err)
	}

	readmePath := filepath.Join(config.Dir(tempHome), "README.md")
	if data, err := os.ReadFile(readmePath); err != nil || len(data) == 0 {
		t.Fatalf("Expected ~/.yups/README.md to exist, err: %v", err)
	}

	// Legacy files should have been removed
	if _, err := os.Stat(legacyData); !os.IsNotExist(err) {
		t.Errorf("Expected legacy file %s to be cleaned up, stat: %v", legacyData, err)
	}
	if _, err := os.Stat(legacyPrompt); !os.IsNotExist(err) {
		t.Errorf("Expected legacy file %s to be cleaned up, stat: %v", legacyPrompt, err)
	}
}

func TestEnsureAssetsUpdated(t *testing.T) {
	tempHome := t.TempDir()

	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
		Remove:      os.Remove,
		PathExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}

	// First ensure writes README.md and shell scripts
	if err := EnsureAssetsUpdated(env, tempHome); err != nil {
		t.Fatalf("EnsureAssetsUpdated failed: %v", err)
	}

	readmePath := filepath.Join(config.Dir(tempHome), "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatalf("README.md not created by EnsureAssetsUpdated: %v", err)
	}

	yupsBash := filepath.Join(config.ShellDir(tempHome), "yups.bash")
	if _, err := os.Stat(yupsBash); err != nil {
		t.Fatalf("yups.bash not created by EnsureAssetsUpdated: %v", err)
	}

	// Override home in assets package to test loading from disk when user creates a custom file
	assets.SetOverrideHome(tempHome)
	defer assets.ResetOverrideHome()

	// Modify on-disk theme to verify dynamic loading
	themeDir := config.DataDir(tempHome)
	_ = os.MkdirAll(themeDir, 0o755)
	themePath := filepath.Join(themeDir, "theme.toml")
	customTheme := "error = \"\\x1b[31m\"\nwarning = \"\\x1b[33m\"\n"
	if err := os.WriteFile(themePath, []byte(customTheme), 0o644); err != nil {
		t.Fatalf("Failed writing custom theme: %v", err)
	}

	if got := assets.GetThemeData(); got != customTheme {
		t.Errorf("GetThemeData() = %q, want %q", got, customTheme)
	}
}
