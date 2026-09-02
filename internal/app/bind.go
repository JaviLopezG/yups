package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"yups/assets"
	"yups/internal/config"
	"yups/internal/ui"
)

const (
	BashIntegrationHeader = "# >>> yups shell integration >>>"
	BashIntegrationFooter = "# <<< yups shell integration <<<"
	// Legacy headers for backward compatibility during upgrades
	LegacyBashBindingHeader = "# >>> yups readline binding >>>"
	LegacyBashBindingFooter = "# <<< yups readline binding <<<"
)

// Standard Readline escape sequences
const (
	SeqF1    = `\eOP`
	SeqF1Alt = `\e[11~`
	SeqCtrlG = `\C-g`
	SeqCtrlH = `\C-h`
)

// GenerateMainShellScriptContent returns the master entrypoint script ~/.yups/shell/yups.bash.
func GenerateMainShellScriptContent() string {
	return assets.ShellYupsBash
}

// GenerateEnvScriptContent returns the command wrapper script ~/.yups/shell/env.bash.
func GenerateEnvScriptContent() string {
	return assets.ShellEnvBash
}

// GenerateCompletionScriptContent returns the tab completion script ~/.yups/shell/completion.bash.
func GenerateCompletionScriptContent() string {
	return assets.ShellCompletionBash
}

// GenerateKeybindingScriptContent creates ~/.yups/shell/keybinding.bash with readline binding and key code cheat sheet.
func GenerateKeybindingScriptContent(seq string) string {
	activeKeyDesc := "(none configured / disabled)"
	bindDirective := "# bind -x '\"\\eOP\": _yups-readline-binding'  # Uncomment to enable"
	if seq != "" {
		keyName := SequenceToKeyName(seq)
		activeKeyDesc = fmt.Sprintf("%s (escape sequence: %s)", keyName, seq)
		bindDirective = fmt.Sprintf("bind -x '\"%s\": _yups-readline-binding'", seq)
	}

	return fmt.Sprintf(assets.ShellKeybindingBash, activeKeyDesc, bindDirective)
}

// GenerateShellScriptContent returns the main bash integration script entrypoint.
func GenerateShellScriptContent(seq string) string {
	return GenerateMainShellScriptContent()
}

// GenerateBashrcSourceBlock creates the small source snippet placed in ~/.bashrc.
func GenerateBashrcSourceBlock(shellScriptPath string) string {
	return fmt.Sprintf(`%s
if [ -f "%s" ]; then
    source "%s"
fi
%s`, BashIntegrationHeader, shellScriptPath, shellScriptPath, BashIntegrationFooter)
}

// ParseInUseKeybindings parses the raw text from 'bind -p; bind -X' into a map of seq -> target.
func ParseInUseKeybindings(output string) map[string]string {
	inUse := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "(not bound)") {
			continue
		}

		var seq, target string
		if strings.Contains(line, ":") {
			// Format from bind -p: "seq": target
			parts := strings.SplitN(line, ":", 2)
			seq = strings.Trim(strings.TrimSpace(parts[0]), "\"")
			target = strings.Trim(strings.TrimSpace(parts[1]), "\"")
		} else {
			// Format from bind -X: "seq" "target"
			if strings.HasPrefix(line, "\"") {
				rest := line[1:]
				endQ := strings.Index(rest, "\"")
				if endQ != -1 {
					seq = rest[:endQ]
					target = strings.Trim(strings.TrimSpace(rest[endQ+1:]), "\"")
				}
			} else {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					seq = strings.Trim(fields[0], "\"")
					target = strings.Trim(strings.Join(fields[1:], " "), "\"")
				}
			}
		}

		if seq == "" || target == "" || target == "self-insert" {
			continue
		}
		inUse[seq] = target
	}
	return inUse
}

// GetInUseKeybindings probes bash's currently active keybindings.
func GetInUseKeybindings(env *Env) map[string]string {
	if env == nil {
		return make(map[string]string)
	}

	inUse := make(map[string]string)

	if env.RunCmdTimeout != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Run bash in interactive mode (-i) so ~/.bashrc bindings are loaded
		out, err := env.RunCmdTimeout(ctx, 2*time.Second, "bash", "-i", "-c", "bind -X; bind -p")
		if err == nil && len(out) > 0 {
			for k, v := range ParseInUseKeybindings(string(out)) {
				inUse[k] = v
			}
		} else {
			// Fallback without -i
			out2, err2 := env.RunCmdTimeout(ctx, 2*time.Second, "bash", "-c", "bind -X; bind -p")
			if err2 == nil && len(out2) > 0 {
				for k, v := range ParseInUseKeybindings(string(out2)) {
					inUse[k] = v
				}
			}
		}
	}

	// Also inspect user's ~/.bashrc directly if available
	if env.UserHomeDir != nil && env.ReadFile != nil {
		if home, err := env.UserHomeDir(); err == nil {
			rcPath := filepath.Join(home, ".bashrc")
			if data, err := env.ReadFile(rcPath); err == nil {
				parseFileBindings(string(data), inUse)
			}
		}
	}

	return inUse
}

func parseFileBindings(content string, inUse map[string]string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "bind ") || strings.HasPrefix(line, "bind -x ") {
			start := strings.Index(line, "\"")
			if start != -1 {
				rest := line[start+1:]
				end := strings.Index(rest, "\"")
				if end != -1 {
					seq := rest[:end]
					// Extract function/target
					target := strings.TrimSpace(rest[end+1:])
					target = strings.TrimPrefix(target, ":")
					target = strings.Trim(strings.TrimSpace(target), "\"'")
					if seq != "" && target != "" && target != "self-insert" {
						inUse[seq] = target
					}
				}
			}
		}
	}
}

// SelectBestKeybinding evaluates available keys in priority order (F1 -> Ctrl+g -> Ctrl+h).
func SelectBestKeybinding(inUse map[string]string) (name string, seq string) {
	// 1. Check F1 (\eOP and \e[11~)
	f1Used := isKeySeqInUse(SeqF1, inUse) || isKeySeqInUse(SeqF1Alt, inUse)
	if !f1Used {
		return "F1", SeqF1
	}

	// 2. Check Ctrl+g (\C-g)
	if !isKeySeqInUse(SeqCtrlG, inUse) {
		return "Ctrl+g", SeqCtrlG
	}

	// 3. Check Ctrl+h (\C-h)
	if !isKeySeqInUse(SeqCtrlH, inUse) {
		return "Ctrl+h", SeqCtrlH
	}

	// All defaults are busy
	return "", ""
}

func isKeySeqInUse(seq string, inUse map[string]string) bool {
	target, exists := inUse[seq]
	if !exists {
		return false
	}
	// If already bound to our own yups function, it is not considered an external conflict
	if target == "_yups-readline-binding" || target == "_yups_readline_binding" || strings.Contains(target, "yups") {
		return false
	}
	return true
}

// KeyNameToSequence translates human-friendly key names or raw input to Readline escape sequences.
func KeyNameToSequence(name string) string {
	clean := strings.ToLower(strings.TrimSpace(name))
	switch clean {
	case "f1":
		return SeqF1
	case "f2":
		return `\eOQ`
	case "f3":
		return `\eOR`
	case "f4":
		return `\eOS`
	case "f5":
		return `\e[15~`
	case "f6":
		return `\e[17~`
	case "f7":
		return `\e[18~`
	case "f8":
		return `\e[19~`
	case "f9":
		return `\e[20~`
	case "f10":
		return `\e[21~`
	case "f11":
		return `\e[23~`
	case "f12":
		return `\e[24~`
	}

	if strings.HasPrefix(clean, "ctrl+") || strings.HasPrefix(clean, "ctrl-") || strings.HasPrefix(clean, "c-") {
		letter := strings.TrimPrefix(clean, "ctrl+")
		letter = strings.TrimPrefix(letter, "ctrl-")
		letter = strings.TrimPrefix(letter, "c-")
		if len(letter) == 1 {
			return `\C-` + letter
		}
	}

	if strings.HasPrefix(clean, "alt+") || strings.HasPrefix(clean, "alt-") || strings.HasPrefix(clean, "m-") {
		letter := strings.TrimPrefix(clean, "alt+")
		letter = strings.TrimPrefix(letter, "alt-")
		letter = strings.TrimPrefix(letter, "m-")
		if len(letter) == 1 {
			return `\e` + letter
		}
	}

	return name
}

// SequenceToKeyName translates Readline sequences back to human-friendly key names where possible.
func SequenceToKeyName(seq string) string {
	switch seq {
	case SeqF1, SeqF1Alt, `\e[[A`:
		return "F1"
	case `\eOQ`, `\e[12~`, `\e[[B`:
		return "F2"
	case `\eOR`, `\e[13~`, `\e[[C`:
		return "F3"
	case `\eOS`, `\e[14~`, `\e[[D`:
		return "F4"
	case `\e[15~`:
		return "F5"
	case `\e[17~`:
		return "F6"
	case `\e[18~`:
		return "F7"
	case `\e[19~`:
		return "F8"
	case `\e[20~`:
		return "F9"
	case `\e[21~`:
		return "F10"
	case `\e[23~`:
		return "F11"
	case `\e[24~`:
		return "F12"
	}
	if strings.HasPrefix(seq, `\C-`) && len(seq) == 4 {
		return "Ctrl+" + string(seq[3])
	}
	if strings.HasPrefix(seq, `\e`) && len(seq) == 3 {
		return "Alt+" + string(seq[2])
	}
	return seq
}

// DecodeRawBytes translates raw terminal keypress bytes into a friendly name and Readline sequence.
func DecodeRawBytes(b []byte) (name string, seq string, isEnter bool, isCancel bool) {
	if len(b) == 0 {
		return "", "", false, false
	}

	// Enter / Return
	if len(b) == 1 && (b[0] == '\r' || b[0] == '\n') {
		return "Enter", "", true, false
	}

	// Ctrl+C (0x03)
	if len(b) == 1 && b[0] == 3 {
		return "Ctrl+C", "", false, true
	}

	// Escape sequences (function keys, Alt combos)
	if b[0] == 0x1b {
		s := string(b)
		switch s {
		case "\x1bOP", "\x1b[11~", "\x1b[[A":
			return "F1", SeqF1, false, false
		case "\x1bOQ", "\x1b[12~", "\x1b[[B":
			return "F2", `\eOQ`, false, false
		case "\x1bOR", "\x1b[13~", "\x1b[[C":
			return "F3", `\eOR`, false, false
		case "\x1bOS", "\x1b[14~", "\x1b[[D":
			return "F4", `\eOS`, false, false
		case "\x1b[15~":
			return "F5", `\e[15~`, false, false
		case "\x1b[17~":
			return "F6", `\e[17~`, false, false
		case "\x1b[18~":
			return "F7", `\e[18~`, false, false
		case "\x1b[19~":
			return "F8", `\e[19~`, false, false
		case "\x1b[20~":
			return "F9", `\e[20~`, false, false
		case "\x1b[21~":
			return "F10", `\e[21~`, false, false
		case "\x1b[23~":
			return "F11", `\e[23~`, false, false
		case "\x1b[24~":
			return "F12", `\e[24~`, false, false
		}
		if len(b) == 2 && b[1] >= 'a' && b[1] <= 'z' {
			return "Alt+" + string(b[1]), `\e` + string(b[1]), false, false
		}
		if len(b) == 2 && b[1] >= 'A' && b[1] <= 'Z' {
			return "Alt+" + strings.ToLower(string(b[1])), `\e` + strings.ToLower(string(b[1])), false, false
		}
		// Generic escape sequence
		escaped := strings.ReplaceAll(s, "\x1b", `\e`)
		return escaped, escaped, false, false
	}

	// Single byte control characters 0x01 - 0x1A
	if len(b) == 1 && b[0] >= 1 && b[0] <= 26 {
		char := string('a' + (b[0] - 1))
		return "Ctrl+" + char, `\C-` + char, false, false
	}

	// Plain printable characters
	if len(b) == 1 && b[0] >= 32 && b[0] <= 126 {
		return string(b), string(b), false, false
	}

	return string(b), string(b), false, false
}

// ValidateKeybinding checks if the user's requested key sequence is already assigned in bash.
func ValidateKeybinding(input string, inUse map[string]string) (seq string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("key combination cannot be empty")
	}

	seq = KeyNameToSequence(trimmed)
	if isKeySeqInUse(seq, inUse) {
		return seq, fmt.Errorf("key '%s' (%s) is already in use by bash (%s)", input, seq, inUse[seq])
	}
	return seq, nil
}

// makeRaw configures the given file descriptor into raw terminal mode.
func makeRaw(fd int) (*syscall.Termios, error) {
	var old syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&old)), 0, 0, 0); err != 0 {
		return nil, err
	}
	raw := old
	raw.Iflag &^= (syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= (syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN)
	raw.Cflag &^= (syscall.CSIZE | syscall.PARENB)
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(&raw)), 0, 0, 0); err != 0 {
		return nil, err
	}
	return &old, nil
}

// restoreTerminal restores the terminal to its previous state.
func restoreTerminal(fd int, old *syscall.Termios) {
	if old != nil {
		_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(old)), 0, 0, 0)
	}
}

// CaptureKeybindingInteractively captures raw keypresses or prompts text fallback.
func CaptureKeybindingInteractively(env *Env, inUse map[string]string, sugName, sugSeq string, stdout, stderr io.Writer) (chosenName string, chosenSeq string) {
	isTTY := env.IsTerminalOutput != nil && env.IsTerminalOutput(stdout)

	var defaultDisplay string
	if sugName != "" {
		defaultDisplay = sugName
	} else {
		defaultDisplay = "none"
	}

	if isTTY {
		stdinFd := int(os.Stdin.Fd())
		oldTerm, err := makeRaw(stdinFd)
		if err == nil {
			defer restoreTerminal(stdinFd, oldTerm)

			fmt.Fprintf(stdout, "Press desired key combination (or Enter to accept [%s]): ", defaultDisplay)

			currentSeq := sugSeq
			currentName := sugName

			buf := make([]byte, 32)
			for {
				n, readErr := os.Stdin.Read(buf)
				if readErr != nil || n == 0 {
					break
				}
				keyName, seq, isEnter, isCancel := DecodeRawBytes(buf[:n])

				if isCancel {
					fmt.Fprint(stdout, "\r\x1b[KCancelled key binding configuration.\r\n")
					return "", ""
				}

				if isEnter {
					if currentSeq != "" && !isKeySeqInUse(currentSeq, inUse) {
						fmt.Fprintf(stdout, "\r\x1b[KSelected key: %s (%s)\r\n", currentName, currentSeq)
						return currentName, currentSeq
					}
					if currentSeq == "" {
						fmt.Fprint(stdout, "\r\x1b[KNo key selected. Skipped.\r\n")
						return "", ""
					}
					// If in use, notify user and keep waiting
					fmt.Fprintf(stdout, "\r\x1b[KKey %s (%s) is already in use by %q. Please press another key: ", currentName, currentSeq, inUse[currentSeq])
					continue
				}

				if seq != "" {
					currentSeq = seq
					currentName = keyName
					theme := ui.GetTheme()
					if isKeySeqInUse(seq, inUse) {
						fmt.Fprintf(stdout, "\r\x1b[KSelected: %s (%s) %s[In use by %q - press another]%s", keyName, seq, theme.Warning, inUse[seq], theme.Reset)
					} else {
						fmt.Fprintf(stdout, "\r\x1b[KSelected: %s (%s) %s[Available - press Enter to confirm]%s", keyName, seq, theme.Success, theme.Reset)
					}
				}
			}
		}
	}

	// Non-TTY / scripted prompt fallback
	if env.AskPrompt != nil {
		prompt := fmt.Sprintf("Key binding for yups [%s]", defaultDisplay)
		for attempt := 0; attempt < 3; attempt++ {
			choice := strings.TrimSpace(env.AskPrompt(prompt, defaultDisplay))
			if choice == "" || choice == defaultDisplay {
				if sugSeq != "" && !isKeySeqInUse(sugSeq, inUse) {
					return sugName, sugSeq
				}
			}
			seq, err := ValidateKeybinding(choice, inUse)
			if err != nil {
				fmt.Fprintf(stderr, "Warning: %v. Please choose another.\n", err)
				continue
			}
			return choice, seq
		}
	}

	return sugName, sugSeq
}

func extractBoundSequence(content, defaultSeq string) string {
	startIdx := strings.Index(content, `bind -x '"`)
	if startIdx != -1 {
		sub := content[startIdx+len(`bind -x '"`):]
		if endQ := strings.Index(sub, `":`); endQ != -1 {
			return sub[:endQ]
		}
	}
	return defaultSeq
}

// InstallBashBinding writes ~/.yups/shell/ modular scripts and updates ~/.bashrc.
func InstallBashBinding(env *Env, home string, seq string) (string, error) {
	// 1. Create ~/.yups/shell directory
	shellDir := config.ShellDir(home)
	_ = os.MkdirAll(shellDir, 0o755)

	// 2. Write ~/.yups/shell/yups.bash (main entrypoint loader)
	mainScript := config.ShellScriptPath(home)
	if env.WriteFile != nil {
		if err := env.WriteFile(mainScript, []byte(GenerateMainShellScriptContent()), 0o755); err != nil {
			return "", fmt.Errorf("writing main shell script %s: %w", mainScript, err)
		}
	}

	// 3. Write ~/.yups/shell/keybinding.bash
	keybindingScript := config.KeybindingScriptPath(home)
	if env.WriteFile != nil {
		if err := env.WriteFile(keybindingScript, []byte(GenerateKeybindingScriptContent(seq)), 0o755); err != nil {
			return "", fmt.Errorf("writing keybinding script %s: %w", keybindingScript, err)
		}
	}

	// 4. Write ~/.yups/shell/completion.bash
	completionScript := config.CompletionScriptPath(home)
	if env.WriteFile != nil {
		if err := env.WriteFile(completionScript, []byte(GenerateCompletionScriptContent()), 0o755); err != nil {
			return "", fmt.Errorf("writing completion script %s: %w", completionScript, err)
		}
	}

	// 5. Write ~/.yups/shell/env.bash
	envScript := config.EnvScriptPath(home)
	if env.WriteFile != nil {
		if err := env.WriteFile(envScript, []byte(GenerateEnvScriptContent()), 0o755); err != nil {
			return "", fmt.Errorf("writing env script %s: %w", envScript, err)
		}
	}

	// 6. Insert source snippet into ~/.bashrc (or ~/.bash_profile / ~/.profile)
	rcPath := filepath.Join(home, ".bashrc")
	if env.PathExists != nil && !env.PathExists(rcPath) {
		profile := filepath.Join(home, ".bash_profile")
		if env.PathExists(profile) {
			rcPath = profile
		} else {
			p := filepath.Join(home, ".profile")
			if env.PathExists(p) {
				rcPath = p
			}
		}
	}

	var existingContent string
	if env.ReadFile != nil {
		data, err := env.ReadFile(rcPath)
		if err == nil {
			existingContent = string(data)
		}
	}

	sourceBlock := GenerateBashrcSourceBlock(mainScript)

	var updatedContent string
	// Check for current integration header or legacy readline header
	startIdx := strings.Index(existingContent, BashIntegrationHeader)
	endIdx := strings.Index(existingContent, BashIntegrationFooter)
	footerLen := len(BashIntegrationFooter)

	if startIdx == -1 {
		startIdx = strings.Index(existingContent, LegacyBashBindingHeader)
		endIdx = strings.Index(existingContent, LegacyBashBindingFooter)
		footerLen = len(LegacyBashBindingFooter)
	}

	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		// Replace existing block cleanly
		endBlock := endIdx + footerLen
		updatedContent = existingContent[:startIdx] + sourceBlock + existingContent[endBlock:]
	} else {
		// Append to file
		if existingContent == "" || strings.HasSuffix(existingContent, "\n") {
			updatedContent = existingContent + sourceBlock + "\n"
		} else {
			updatedContent = existingContent + "\n\n" + sourceBlock + "\n"
		}
	}

	if env.WriteFile != nil {
		if err := env.WriteFile(rcPath, []byte(updatedContent), 0o644); err != nil {
			return rcPath, fmt.Errorf("writing %s: %w", rcPath, err)
		}
	}
	return rcPath, nil
}

// ConfigureBashBindingInteractively prompts the user and sets up the bash binding if desired.
func ConfigureBashBindingInteractively(env *Env, home string, stdout, stderr io.Writer) {
	if env == nil || env.AskConfirmation == nil {
		return
	}

	prompt := "Do you want to configure a bash key binding for yups (runs yups on current command line)?"
	if !env.AskConfirmation(prompt, true) {
		return
	}

	inUse := GetInUseKeybindings(env)
	sugName, sugSeq := SelectBestKeybinding(inUse)

	chosenDisplay, chosenSeq := CaptureKeybindingInteractively(env, inUse, sugName, sugSeq, stdout, stderr)

	if chosenSeq == "" {
		fmt.Fprintln(stdout, "Bash key binding configuration skipped.")
		return
	}

	rcPath, err := InstallBashBinding(env, home, chosenSeq)
	if err != nil {
		fmt.Fprintf(stderr, "Could not configure key binding: %v\n", err)
		return
	}

	if env.LoadUpdateState != nil && env.SaveUpdateState != nil {
		st, _ := env.LoadUpdateState(config.StatePath(home))
		st.Keybinding = chosenDisplay
		_ = env.SaveUpdateState(config.StatePath(home), st)
	}

	fmt.Fprintf(stdout, "\nConfigured key binding (%s) in %s.\nUpdated %s with YUPS shell integration.\nRun 'source %s' or restart your terminal to activate it.\n",
		chosenDisplay, config.ShellScriptPath(home), rcPath, rcPath)
}

// EnsureBashBindingUpdated updates an existing binding script silently or prompts if not configured yet.
func EnsureBashBindingUpdated(env *Env, home string, stdout, stderr io.Writer) {
	shellScript := config.ShellScriptPath(home)
	keybindingScript := config.KeybindingScriptPath(home)
	rcPath := filepath.Join(home, ".bashrc")
	if env.PathExists != nil && !env.PathExists(rcPath) {
		profile := filepath.Join(home, ".bash_profile")
		if env.PathExists(profile) {
			rcPath = profile
		} else {
			p := filepath.Join(home, ".profile")
			if env.PathExists(p) {
				rcPath = p
			}
		}
	}

	var existingContent string
	if env.ReadFile != nil {
		data, err := env.ReadFile(rcPath)
		if err == nil {
			existingContent = string(data)
		}
	}

	hasIntegration := strings.Contains(existingContent, BashIntegrationHeader) || strings.Contains(existingContent, LegacyBashBindingHeader)
	if hasIntegration {
		// Read existing script to keep current key sequence
		seq := SeqF1
		if env.ReadFile != nil {
			if data, err := env.ReadFile(keybindingScript); err == nil {
				seq = extractBoundSequence(string(data), seq)
			} else if data, err := env.ReadFile(shellScript); err == nil {
				seq = extractBoundSequence(string(data), seq)
			}
		}
		_, _ = InstallBashBinding(env, home, seq)
		return
	}

	ConfigureBashBindingInteractively(env, home, stdout, stderr)
}

// RemoveBashBinding cleans up the yups shell integration block from the user's rc file.
func RemoveBashBinding(env *Env, home string) bool {
	rcFiles := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}

	removedAny := false
	for _, rcPath := range rcFiles {
		if env.PathExists != nil && !env.PathExists(rcPath) {
			continue
		}

		var content string
		if env.ReadFile != nil {
			data, err := env.ReadFile(rcPath)
			if err != nil {
				continue
			}
			content = string(data)
		} else {
			continue
		}

		startIdx := strings.Index(content, BashIntegrationHeader)
		endIdx := strings.Index(content, BashIntegrationFooter)
		footerLen := len(BashIntegrationFooter)

		if startIdx == -1 {
			startIdx = strings.Index(content, LegacyBashBindingHeader)
			endIdx = strings.Index(content, LegacyBashBindingFooter)
			footerLen = len(LegacyBashBindingFooter)
		}

		if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
			endBlock := endIdx + footerLen
			cleaned := content[:startIdx] + content[endBlock:]
			cleaned = strings.TrimRight(cleaned, "\n")
			if cleaned != "" {
				cleaned += "\n"
			}
			if env.WriteFile != nil {
				_ = env.WriteFile(rcPath, []byte(cleaned), 0o644)
			}
			removedAny = true
		}
	}
	if removedAny && env.LoadUpdateState != nil && env.SaveUpdateState != nil {
		st, _ := env.LoadUpdateState(config.StatePath(home))
		if st.Keybinding != "" {
			st.Keybinding = ""
			_ = env.SaveUpdateState(config.StatePath(home), st)
		}
	}
	return removedAny
}
