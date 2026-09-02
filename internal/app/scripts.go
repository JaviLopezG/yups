package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"yups/assets"
	"yups/internal/config"
)

// InspectSessionPyContent is the full python script for inspecting session logs.
var InspectSessionPyContent = assets.InspectSessionPy

// InstallScripts writes inspect-session.py to ~/.yups/scripts/.
func InstallScripts(env *Env, home string) error {
	scriptsDir := config.ScriptsDir(home)
	_ = os.MkdirAll(scriptsDir, 0o755)

	pyPath := filepath.Join(scriptsDir, "inspect-session.py")

	if env.WriteFile != nil {
		if err := env.WriteFile(pyPath, []byte(InspectSessionPyContent), 0o755); err != nil {
			return fmt.Errorf("writing script %s: %w", pyPath, err)
		}
	}

	// Clean up legacy wrapper if present
	legacySh := filepath.Join(scriptsDir, "inspect-session.sh")
	if env.Remove != nil {
		_ = env.Remove(legacySh)
	}

	return nil
}

// EnsureScriptsUpdated updates inspect-session.py in ~/.yups/scripts/
// if it is missing or differs from the current binary's embedded script.
func EnsureScriptsUpdated(env *Env, home string) error {
	scriptsDir := config.ScriptsDir(home)
	pyPath := filepath.Join(scriptsDir, "inspect-session.py")

	needsPyUpdate := true

	if env.ReadFile != nil {
		if data, err := env.ReadFile(pyPath); err == nil && bytes.Equal(data, []byte(InspectSessionPyContent)) {
			needsPyUpdate = false
		}
	}

	if !needsPyUpdate {
		return nil
	}

	return InstallScripts(env, home)
}
