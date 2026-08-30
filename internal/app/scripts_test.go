package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yups/internal/config"
)

func TestInstallScriptsWritesScripts(t *testing.T) {
	tempHome := t.TempDir()
	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
		Remove:      os.Remove,
	}

	if err := InstallScripts(env, tempHome); err != nil {
		t.Fatalf("InstallScripts failed: %v", err)
	}

	scriptsDir := config.ScriptsDir(tempHome)
	pyPath := filepath.Join(scriptsDir, "inspect-session.py")

	pyData, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("reading inspect-session.py: %v", err)
	}
	if !strings.Contains(string(pyData), "inspect-session.py") {
		t.Errorf("inspect-session.py missing header")
	}
}

func TestEnsureScriptsUpdatedReplacesOutdated(t *testing.T) {
	tempHome := t.TempDir()
	env := &Env{
		UserHomeDir: func() (string, error) { return tempHome, nil },
		WriteFile:   os.WriteFile,
		ReadFile:    os.ReadFile,
		Remove:      os.Remove,
	}

	scriptsDir := config.ScriptsDir(tempHome)
	_ = os.MkdirAll(scriptsDir, 0o755)
	pyPath := filepath.Join(scriptsDir, "inspect-session.py")
	_ = os.WriteFile(pyPath, []byte("old script"), 0o755)

	if err := EnsureScriptsUpdated(env, tempHome); err != nil {
		t.Fatalf("EnsureScriptsUpdated failed: %v", err)
	}

	data, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("reading updated inspect-session.py: %v", err)
	}
	if bytes.Equal(data, []byte("old script")) {
		t.Errorf("expected script to be updated, still old script")
	}
	if !strings.Contains(string(data), "inspect-session.py") {
		t.Errorf("updated script missing content")
	}
}
