package app

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yups/internal/config"
)

func TestParseInUseKeybindings(t *testing.T) {
	output := `
# bash readline bindings
"\C-a": beginning-of-line
"\C-b": backward-char
"\C-d": delete-char
"\C-f": forward-char
"\C-g": abort
"\C-h": backward-delete-char
"\eOP" "explain_current_line"
"\e[11~": self-insert
"\C-x\C-u": undo
# "\C-z": (not bound)
`
	inUse := ParseInUseKeybindings(output)

	if inUse[`\C-a`] != "beginning-of-line" {
		t.Errorf(`inUse[\C-a] = %q, want "beginning-of-line"`, inUse[`\C-a`])
	}
	if inUse[`\C-g`] != "abort" {
		t.Errorf(`inUse[\C-g] = %q, want "abort"`, inUse[`\C-g`])
	}
	if inUse[`\eOP`] != "explain_current_line" {
		t.Errorf(`inUse[\eOP] = %q, want "explain_current_line"`, inUse[`\eOP`])
	}
	// self-insert and not bound should not be considered in use
	if _, ok := inUse[`\e[11~`]; ok {
		t.Errorf(`inUse[\e[11~] should be free, got %q`, inUse[`\e[11~`])
	}
	if _, ok := inUse[`\C-z`]; ok {
		t.Errorf(`inUse[\C-z] should be free, got %q`, inUse[`\C-z`])
	}
}

func TestDecodeRawBytes(t *testing.T) {
	t.Run("f1-escape-sequence", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte("\x1bOP"))
		if name != "F1" || seq != SeqF1 || isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want (F1, %s, false, false)", name, seq, isEnter, isCancel, SeqF1)
		}
	})

	t.Run("ctrl-g-byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{7})
		if name != "Ctrl+g" || seq != SeqCtrlG || isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want (Ctrl+g, %s, false, false)", name, seq, isEnter, isCancel, SeqCtrlG)
		}
	})

	t.Run("enter-byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{'\r'})
		if !isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want enter", name, seq, isEnter, isCancel)
		}
	})

	t.Run("ctrl-c-byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{3})
		if !isCancel {
			t.Errorf("got (%q, %q, %v, %v), want cancel", name, seq, isEnter, isCancel)
		}
	})
}

func TestSelectBestKeybinding(t *testing.T) {
	t.Run("f1-available-when-map-empty", func(t *testing.T) {
		name, seq := SelectBestKeybinding(map[string]string{})
		if name != "F1" || seq != SeqF1 {
			t.Errorf("got (%q, %q), want (F1, %s)", name, seq, SeqF1)
		}
	})

	t.Run("f1-busy-suggests-ctrl-g", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1: "explain_current_line",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "Ctrl+g" || seq != SeqCtrlG {
			t.Errorf("got (%q, %q), want (Ctrl+g, %s)", name, seq, SeqCtrlG)
		}
	})

	t.Run("f1-and-ctrl-g-busy-suggests-ctrl-h", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1:    "explain_current_line",
			SeqCtrlG: "abort",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "Ctrl+h" || seq != SeqCtrlH {
			t.Errorf("got (%q, %q), want (Ctrl+h, %s)", name, seq, SeqCtrlH)
		}
	})

	t.Run("all-defaults-busy-returns-empty", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1:    "explain_current_line",
			SeqCtrlG: "abort",
			SeqCtrlH: "backward-delete-char",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "" || seq != "" {
			t.Errorf("got (%q, %q), want empty strings", name, seq)
		}
	})

	t.Run("f1-already-bound-to-yups-is-reused", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1: "_yups-readline-binding",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "F1" || seq != SeqF1 {
			t.Errorf("got (%q, %q), want (F1, %s)", name, seq, SeqF1)
		}
	})
}

func TestValidateKeybinding(t *testing.T) {
	inUse := map[string]string{
		`\eOP`: "explain_current_line",
		`\C-g`: "abort",
	}

	// Busy keys should fail
	if _, err := ValidateKeybinding("F1", inUse); err == nil {
		t.Error("expected F1 to fail validation when busy")
	}
	if _, err := ValidateKeybinding("ctrl+g", inUse); err == nil {
		t.Error("expected ctrl+g to fail validation when busy")
	}

	// Free keys should succeed
	seq, err := ValidateKeybinding("F2", inUse)
	if err != nil {
		t.Errorf("F2 validation failed: %v", err)
	}
	if seq != `\eOQ` {
		t.Errorf("F2 sequence = %q, want \\eOQ", seq)
	}

	seq, err = ValidateKeybinding("ctrl+k", inUse)
	if err != nil {
		t.Errorf("ctrl+k validation failed: %v", err)
	}
	if seq != `\C-k` {
		t.Errorf("ctrl+k sequence = %q, want \\C-k", seq)
	}
}

func TestInstallBashBinding(t *testing.T) {
	fs := newFakeFS()
	env := fs.env()

	// Initial installation in clean .bashrc
	rcPath, err := InstallBashBinding(env, fs.home, SeqF1)
	if err != nil {
		t.Fatalf("InstallBashBinding: %v", err)
	}

	// 1. Verify main loader script in ~/.yups/shell/yups.bash
	shellScript := config.ShellScriptPath(fs.home)
	mainContent, ok := fs.fileContents[shellScript]
	if !ok {
		t.Fatalf("main shell script was not written to %s", shellScript)
	}
	if !strings.Contains(mainContent, "# yups.bash - Main entrypoint for YUPS shell integration.") {
		t.Errorf("main script missing expected header:\n%s", mainContent)
	}
	if !strings.Contains(mainContent, "source \"$_yups_script\"") {
		t.Errorf("main script missing sourcing loop:\n%s", mainContent)
	}

	// 2. Verify keybinding script in ~/.yups/shell/keybinding.bash
	keyScript := config.KeybindingScriptPath(fs.home)
	keyContent, ok := fs.fileContents[keyScript]
	if !ok {
		t.Fatalf("keybinding script was not written to %s", keyScript)
	}
	if !strings.Contains(keyContent, "# keybinding.bash - Readline key binding integration for yups.") {
		t.Errorf("keybinding script missing expected header:\n%s", keyContent)
	}
	if !strings.Contains(keyContent, "_yups-readline-binding") || !strings.Contains(keyContent, SeqF1) {
		t.Errorf("keybinding script missing binding content:\n%s", keyContent)
	}
	if !strings.Contains(keyContent, "COMMON KEY CODES REFERENCE TABLE") {
		t.Errorf("keybinding script missing key codes reference table:\n%s", keyContent)
	}
	if !strings.Contains(keyContent, "Active key: F1") {
		t.Errorf("keybinding script missing active key indicator:\n%s", keyContent)
	}

	// 3. Verify completion script in ~/.yups/shell/completion.bash
	compScript := config.CompletionScriptPath(fs.home)
	compContent, ok := fs.fileContents[compScript]
	if !ok {
		t.Fatalf("completion script was not written to %s", compScript)
	}
	if !strings.Contains(compContent, "# completion.bash - Programmable tab-completion for yups.") {
		t.Errorf("completion script missing expected header:\n%s", compContent)
	}
	if !strings.Contains(compContent, "_yups-completion") || !strings.Contains(compContent, "complete -F _yups-completion yups") {
		t.Errorf("completion script missing autocompletion function:\n%s", compContent)
	}

	// 4. Verify env wrapper script in ~/.yups/shell/env.bash
	envScript := config.EnvScriptPath(fs.home)
	envContent, ok := fs.fileContents[envScript]
	if !ok {
		t.Fatalf("env wrapper script was not written to %s", envScript)
	}
	if !strings.Contains(envContent, "# env.bash - Shell wrapper function for the yups command.") {
		t.Errorf("env script missing expected header:\n%s", envContent)
	}
	if !strings.Contains(envContent, "yups()") || !strings.Contains(envContent, "YUPS_SESSION_HISTORY") {
		t.Errorf("env script missing wrapper function:\n%s", envContent)
	}

	// 5. Verify ~/.bashrc has source block
	rcContent := fs.fileContents[rcPath]
	if !strings.Contains(rcContent, BashIntegrationHeader) || !strings.Contains(rcContent, "source \""+shellScript+"\"") {
		t.Errorf("bashrc missing source snippet:\n%s", rcContent)
	}

	// Idempotent re-installation with different key (Ctrl+g) should replace block without duplication
	_, err = InstallBashBinding(env, fs.home, SeqCtrlG)
	if err != nil {
		t.Fatalf("Re-installing binding: %v", err)
	}

	updatedRc := fs.fileContents[rcPath]
	if strings.Count(updatedRc, BashIntegrationHeader) != 1 {
		t.Errorf("expected exactly 1 integration header, got %d:\n%s", strings.Count(updatedRc, BashIntegrationHeader), updatedRc)
	}

	updatedKeyScript := fs.fileContents[keyScript]
	if !strings.Contains(updatedKeyScript, SeqCtrlG) || !strings.Contains(updatedKeyScript, "Active key: Ctrl+g") {
		t.Errorf("keybinding script missing updated sequence %s:\n%s", SeqCtrlG, updatedKeyScript)
	}
}

func TestConfigureBashBindingInteractively(t *testing.T) {
	t.Run("declining-prompt-installs-nothing", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskConfirmation = func(prompt string, defaultYes bool) bool {
			return false
		}

		var stdout, stderr bytes.Buffer
		ConfigureBashBindingInteractively(env, fs.home, &stdout, &stderr)

		shellScript := config.ShellScriptPath(fs.home)
		if _, ok := fs.fileContents[shellScript]; ok {
			t.Errorf("shell script should not have been created when declining")
		}
	})

	t.Run("accepting-installs-suggested-f1", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskConfirmation = func(prompt string, defaultYes bool) bool {
			return true
		}
		env.AskPrompt = func(prompt, defaultValue string) string {
			return defaultValue // accept F1
		}
		env.RunCmdTimeout = func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte(""), nil // no keys busy
		}

		var stdout, stderr bytes.Buffer
		ConfigureBashBindingInteractively(env, fs.home, &stdout, &stderr)

		keyScript := config.KeybindingScriptPath(fs.home)
		keyContent, ok := fs.fileContents[keyScript]
		if !ok {
			t.Fatalf("keybinding script was not written")
		}
		if !strings.Contains(keyContent, SeqF1) {
			t.Errorf("expected SeqF1 in keybinding script:\n%s", keyContent)
		}
		if !strings.Contains(stdout.String(), "Updated /home/user/.bashrc with YUPS shell integration.") {
			t.Errorf("stdout missing confirmation message:\n%s", stdout.String())
		}
	})

	t.Run("accepting-with-f1-busy-installs-ctrl-g", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()
		env.AskConfirmation = func(prompt string, defaultYes bool) bool {
			return true
		}
		env.AskPrompt = func(prompt, defaultValue string) string {
			return defaultValue // accept Ctrl+g
		}
		env.RunCmdTimeout = func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte(`"\eOP" "explain_current_line"`), nil
		}

		var stdout, stderr bytes.Buffer
		ConfigureBashBindingInteractively(env, fs.home, &stdout, &stderr)

		keyScript := config.KeybindingScriptPath(fs.home)
		keyContent, ok := fs.fileContents[keyScript]
		if !ok {
			t.Fatalf("keybinding script was not written")
		}
		if !strings.Contains(keyContent, SeqCtrlG) {
			t.Errorf("expected SeqCtrlG in keybinding script:\n%s", keyContent)
		}
		if !strings.Contains(stdout.String(), "Configured key binding (Ctrl+g)") {
			t.Errorf("stdout missing confirmation message:\n%s", stdout.String())
		}
	})
}

func TestRemoveBashBinding(t *testing.T) {
	fs := newFakeFS()
	env := fs.env()

	// Install binding first
	rcPath, err := InstallBashBinding(env, fs.home, SeqF1)
	if err != nil {
		t.Fatalf("InstallBashBinding: %v", err)
	}
	if !strings.Contains(fs.fileContents[rcPath], BashIntegrationHeader) {
		t.Fatalf("expected integration header in %s", rcPath)
	}

	// Now remove it
	removed := RemoveBashBinding(env, fs.home)
	if !removed {
		t.Errorf("RemoveBashBinding returned false, want true")
	}

	rcClean := fs.fileContents[rcPath]
	if strings.Contains(rcClean, BashIntegrationHeader) || strings.Contains(rcClean, "yups.bash") {
		t.Errorf("bashrc still contains yups integration after removal:\n%s", rcClean)
	}
}

func TestEnsureBashBindingUpdated(t *testing.T) {
	t.Run("preserves-existing-binding-from-keybinding-bash", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		// Initial install with SeqCtrlG
		_, err := InstallBashBinding(env, fs.home, SeqCtrlG)
		if err != nil {
			t.Fatalf("InstallBashBinding: %v", err)
		}

		var stdout, stderr bytes.Buffer
		EnsureBashBindingUpdated(env, fs.home, &stdout, &stderr)

		keyScript := config.KeybindingScriptPath(fs.home)
		keyContent := fs.fileContents[keyScript]
		if !strings.Contains(keyContent, SeqCtrlG) {
			t.Errorf("expected SeqCtrlG to be preserved, got:\n%s", keyContent)
		}
	})

	t.Run("preserves-existing-binding-from-legacy-yups-bash", func(t *testing.T) {
		fs := newFakeFS()
		env := fs.env()

		// Simulate legacy yups.bash having the binding and .bashrc sourced
		shellScript := config.ShellScriptPath(fs.home)
		fs.fileContents[shellScript] = `bind -x '"\C-h": _yups-readline-binding'`
		rcPath := filepath.Join(fs.home, ".bashrc")
		fs.fileContents[rcPath] = GenerateBashrcSourceBlock(shellScript)

		var stdout, stderr bytes.Buffer
		EnsureBashBindingUpdated(env, fs.home, &stdout, &stderr)

		keyScript := config.KeybindingScriptPath(fs.home)
		keyContent := fs.fileContents[keyScript]
		if !strings.Contains(keyContent, SeqCtrlH) {
			t.Errorf("expected SeqCtrlH to be preserved from legacy script, got:\n%s", keyContent)
		}
	})
}

func TestShellScriptsSourcing(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available in PATH")
	}

	tmpDir, err := os.MkdirTemp("", "yups-shell-test-*.kk")
	if err != nil {
		t.Fatalf("os.MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	env := NewOSEnv()
	_, err = InstallBashBinding(env, tmpDir, SeqF1)
	if err != nil {
		t.Fatalf("InstallBashBinding: %v", err)
	}

	mainScript := config.ShellScriptPath(tmpDir)
	cmd := exec.Command("bash", "-c", "source \"$1\" && type _yups-readline-binding && type _yups-completion && type yups", "bash", mainScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sourcing yups.bash failed in bash: %v\nOutput:\n%s", err, string(out))
	}
}
