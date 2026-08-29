package app

import (
	"bytes"
	"context"
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
	t.Run("f1_escape_sequence", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte("\x1bOP"))
		if name != "F1" || seq != SeqF1 || isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want (F1, %s, false, false)", name, seq, isEnter, isCancel, SeqF1)
		}
	})

	t.Run("ctrl_g_byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{7})
		if name != "Ctrl+g" || seq != SeqCtrlG || isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want (Ctrl+g, %s, false, false)", name, seq, isEnter, isCancel, SeqCtrlG)
		}
	})

	t.Run("enter_byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{'\r'})
		if !isEnter || isCancel {
			t.Errorf("got (%q, %q, %v, %v), want enter", name, seq, isEnter, isCancel)
		}
	})

	t.Run("ctrl_c_byte", func(t *testing.T) {
		name, seq, isEnter, isCancel := DecodeRawBytes([]byte{3})
		if !isCancel {
			t.Errorf("got (%q, %q, %v, %v), want cancel", name, seq, isEnter, isCancel)
		}
	})
}

func TestSelectBestKeybinding(t *testing.T) {
	t.Run("f1_available_when_map_empty", func(t *testing.T) {
		name, seq := SelectBestKeybinding(map[string]string{})
		if name != "F1" || seq != SeqF1 {
			t.Errorf("got (%q, %q), want (F1, %s)", name, seq, SeqF1)
		}
	})

	t.Run("f1_busy_suggests_ctrl_g", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1: "explain_current_line",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "Ctrl+g" || seq != SeqCtrlG {
			t.Errorf("got (%q, %q), want (Ctrl+g, %s)", name, seq, SeqCtrlG)
		}
	})

	t.Run("f1_and_ctrl_g_busy_suggests_ctrl_h", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1:    "explain_current_line",
			SeqCtrlG: "abort",
		}
		name, seq := SelectBestKeybinding(inUse)
		if name != "Ctrl+h" || seq != SeqCtrlH {
			t.Errorf("got (%q, %q), want (Ctrl+h, %s)", name, seq, SeqCtrlH)
		}
	})

	t.Run("all_defaults_busy_returns_empty", func(t *testing.T) {
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

	t.Run("f1_already_bound_to_yups_is_reused", func(t *testing.T) {
		inUse := map[string]string{
			SeqF1: "_yups_readline_binding",
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

	// Verify standalone script was created in ~/.yups/shell/yups.bash
	shellScript := config.ShellScriptPath(fs.home)
	scriptContent, ok := fs.fileContents[shellScript]
	if !ok {
		t.Fatalf("shell script was not written to %s", shellScript)
	}
	if !strings.Contains(scriptContent, "_yups_readline_binding") || !strings.Contains(scriptContent, SeqF1) {
		t.Errorf("shell script missing binding content:\n%s", scriptContent)
	}
	if !strings.Contains(scriptContent, "_yups_completion") || !strings.Contains(scriptContent, "complete -F _yups_completion yups") {
		t.Errorf("shell script missing autocompletion function:\n%s", scriptContent)
	}

	// Verify ~/.bashrc has source block
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

	updatedScript := fs.fileContents[shellScript]
	if !strings.Contains(updatedScript, SeqCtrlG) {
		t.Errorf("shell script missing updated sequence %s:\n%s", SeqCtrlG, updatedScript)
	}
}

func TestConfigureBashBindingInteractively(t *testing.T) {
	t.Run("declining_prompt_installs_nothing", func(t *testing.T) {
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

	t.Run("accepting_installs_suggested_f1", func(t *testing.T) {
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

		shellScript := config.ShellScriptPath(fs.home)
		scriptContent, ok := fs.fileContents[shellScript]
		if !ok {
			t.Fatalf("shell script was not written")
		}
		if !strings.Contains(scriptContent, SeqF1) {
			t.Errorf("expected SeqF1 in shell script:\n%s", scriptContent)
		}
		if !strings.Contains(stdout.String(), "Updated /home/user/.bashrc with YUPS shell integration.") {
			t.Errorf("stdout missing confirmation message:\n%s", stdout.String())
		}
	})

	t.Run("accepting_with_f1_busy_installs_ctrl_g", func(t *testing.T) {
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

		shellScript := config.ShellScriptPath(fs.home)
		scriptContent, ok := fs.fileContents[shellScript]
		if !ok {
			t.Fatalf("shell script was not written")
		}
		if !strings.Contains(scriptContent, SeqCtrlG) {
			t.Errorf("expected SeqCtrlG in shell script:\n%s", scriptContent)
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
