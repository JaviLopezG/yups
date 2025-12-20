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

const (
	hookStart = "# --- YUPS_HOOK_START ---"
	hookEnd   = "# --- YUPS_HOOK_END ---"
	yupsPath  = "/usr/local/bin/yups"
)

func init() {
	rootCmd.Flags().BoolVar(&acMode, "auto-config",
		false, "Set configuration to default values.")
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

	if err := updateBashrc(); err != nil {
		slog.Error("Failed to update .bashrc", "error", err)
	} else {
		slog.Info(".bashrc hooks updated successfully")
	}

	installProvidesHelper()
	copyExecutableToPath()
	//TODO manage other shell different of bash
}

func updateBashrc() error {
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

	finalContent := strings.TrimSpace(strings.Join(newLines, "\n")) + "\n" + bashHooks + "\n"
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

func runSudoCommand(name string, args ...string) error {
	allArgs := append([]string{name}, args...)
	cmd := exec.Command("sudo", allArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
