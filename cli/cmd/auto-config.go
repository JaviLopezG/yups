package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/tu-usuario/yups/cli/internal/sys"
)

var acMode bool
var arMode bool
var sudoRunner = actualSudoRunner
var yupsPath = "/usr/local/bin/yups"

const (
	hookStart = "# --- YUPS_HOOK_START ---"
	hookEnd   = "# --- YUPS_HOOK_END ---"
)

func init() {
	rootCmd.Flags().BoolVar(&acMode, "auto-config",
		false, "Set configuration to default values.")

	rootCmd.Flags().BoolVar(&arMode, "auto-remove",
		false, "Remove configuration and binaries.")
}

func handleAR() {
	home, _ := os.UserHomeDir()
	os.RemoveAll(filepath.Join(home, ".yups"))
	updateBashrc(false)
	runSudoCommand("rm", yupsPath)
}

func handleAC() {
	slog.Info("Straw-boss (AC Mode).")

	info := sys.GetSystemInfo()
	viper.Set("os", info.OS)
	viper.Set("pm", info.PM)
	viper.Set("distro_id", info.DistroID)
	viper.Set("distro_version", info.DistroVersion)
	viper.Set("distro_pretty", info.DistroPretty)
	viper.Set("log_level", "info")

	if err := viper.WriteConfig(); err != nil {
		os.MkdirAll(filepath.Dir(viper.ConfigFileUsed()), 0755)
		viper.SafeWriteConfig()
	}

	if err := updateBashrc(true); err != nil {
		slog.Error("Failed to update .bashrc", "error", err)
	} else {
		slog.Info(".bashrc hooks updated successfully")
	}

	installProvidesHelper()
	copyExecutableToPath()
	//TODO manage other shell different of bash
}

func updateBashrc(insert bool) error {
	home, _ := os.UserHomeDir()
	bashrcPath := filepath.Join(home, ".bashrc")

	content, err := os.ReadFile(bashrcPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	skipping := false
	for _, line := range lines {
		if strings.Contains(line, hookStart) {
			skipping = true
			continue
		}
		if strings.Contains(line, hookEnd) {
			skipping = false
			continue
		}
		if !skipping {
			newLines = append(newLines, line)
		}
	}

	bashHooks := fmt.Sprintf(`
%s
# Hooks for the YUPS project
command_not_found_handle() {
    if "%s" --command-not-found "$@"; then
        return $?
    else
        return 127
    fi
}
export -f command_not_found_handle

_yups_ce_handle() {
    local exit_code=$?
    # 130 is Ctrl+C, 127 is CNF (handled above), 0 is success
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]] || [[ $exit_code -eq 130 ]]; then
        return
    fi
    "%s" --command-error "$exit_code" "$YUPS_LAST_CMD"
}
export -f _yups_ce_handle

if [[ -z "$PROMPT_COMMAND" ]]; then
    export PROMPT_COMMAND="_yups_ce_handle"
elif ! [[ "$PROMPT_COMMAND" == *"_yups_ce_handle"* ]]; then
    export PROMPT_COMMAND="_yups_ce_handle;${PROMPT_COMMAND}"
fi

_yups_save_last_cmd() {
    if [[ "$BASH_COMMAND" != "_yups_ce_handle" ]]; then
        export YUPS_LAST_CMD="$BASH_COMMAND"
    fi
}
trap '_yups_save_last_cmd' DEBUG
%s`, hookStart, yupsPath, yupsPath, hookEnd)

	var finalContent string
	if insert {
		finalContent = strings.TrimSpace(strings.Join(newLines, "\n")) + "\n" + bashHooks + "\n"
	} else {
		finalContent = strings.TrimSpace(strings.Join(newLines, "\n"))
	}
	return os.WriteFile(bashrcPath, []byte(finalContent), 0644)
}

func installProvidesHelper() {
	info := sys.GetSystemInfo()

	switch info.PM {
	case "apt":
		if _, err := exec.LookPath("apt-file"); err != nil {
			slog.Info("Installing apt-file for advanced search...")
			runSudoCommand("apt-get", "update")
			runSudoCommand("apt-get", "install", "-y", "apt-file")
			runSudoCommand("apt-file", "update")
		}
	case "pacman":
		if _, err := exec.LookPath("pkgfile"); err != nil {
			slog.Info("Installing pkgfile for advanced search...")
			runSudoCommand("pacman", "-S", "--noconfirm", "pkgfile")
			runSudoCommand("pkgfile", "--update")
		}
	}
}

func copyExecutableToPath() {
	targetPath := yupsPath
	currentPath, err := os.Executable()
	if err != nil {
		slog.Error("Could not determine current executable path", "error", err)
		return
	}

	if currentPath == targetPath {
		return
	}

	slog.Info("Ensuring yups is in /usr/local/bin...", "from", currentPath)

	if err := runSudoCommand("cp", currentPath, targetPath); err != nil {
		slog.Error("Failed to copy executable to path", "error", err)
		return
	}

	runSudoCommand("chmod", "+x", targetPath)
}

func runSudoCommand(name string, args ...string) error {
	return sudoRunner(name, args...)
}

func actualSudoRunner(name string, args ...string) error {
	if !isInteractive() && os.Geteuid() != 0 {
		return fmt.Errorf("non-interactive terminal: sudo requires a TTY or root privileges")
	}

	allArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", allArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
