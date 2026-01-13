package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/javilopezg/yups/cli/internal/sys"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

var acMode bool
var arMode bool
var yupsPath = "/usr/local/bin/yups"
var modelUri = "https://cas-bridge.xethub.hf.co/xet-bridge-us/69442d64307f9531f19aa7b6/ebc700d1689330e46fa2710c4bc5618ce1d991fe3ad952e56e2b4caedcb18aa2?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Content-Sha256=UNSIGNED-PAYLOAD&X-Amz-Credential=cas%2F20251227%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251227T085209Z&X-Amz-Expires=3600&X-Amz-Signature=436ba6ecdff7b4c5174bd70f459d373c675f554b0886462b820a8f8bb2bc8ee4&X-Amz-SignedHeaders=host&X-Xet-Cas-Uid=69225844e51f05ecb1b9574a&response-content-disposition=attachment%3B+filename*%3DUTF-8%27%27functiongemma-270m-it-q8_0.gguf%3B+filename%3D%22functiongemma-270m-it-q8_0.gguf%22%3B&x-id=GetObject&Expires=1766829129&Policy=eyJTdGF0ZW1lbnQiOlt7IkNvbmRpdGlvbiI6eyJEYXRlTGVzc1RoYW4iOnsiQVdTOkVwb2NoVGltZSI6MTc2NjgyOTEyOX19LCJSZXNvdXJjZSI6Imh0dHBzOi8vY2FzLWJyaWRnZS54ZXRodWIuaGYuY28veGV0LWJyaWRnZS11cy82OTQ0MmQ2NDMwN2Y5NTMxZjE5YWE3YjYvZWJjNzAwZDE2ODkzMzBlNDZmYTI3MTBjNGJjNTYxOGNlMWQ5OTFmZTNhZDk1MmU1NmUyYjRjYWVkY2IxOGFhMioifV19&Signature=iP4sZDQu7QH9SHJnJwyJp2TsdRyBvznjqqqMFXuWYTTzZ2baUI607-YEXMVGN7xMy-otSnxamKrVhAgrElIAz%7EfvwDedRN4gUYJzZLfM9oUzKx7l61moB%7EeoYAO01W3pitcZy6wYm-J0ATpSbaK20Kl5PnnaNPAg6YsllexiD-neHDsnEwYOdIRLRWsLWYnTuF2idI2uCPQ-a0zQrkfn6voNMPLah%7E-aeMqtTJX7Mw6swDmWt1HTL7iU%7ER1CQ%7EMxX7VFFSQlaP2V5Q8sPuOiHtH8E442MWZIbrpUXAFBmZ0RX1TdOqBdm1GGyp0RYyOa3ms%7ES9R0H6gbB2fJV3emTQ__&Key-Pair-Id=K2L8F4GPSG1IFC" //https://huggingface.co/bartowski/google_functiongemma-270m-it-GGUF/resolve/main/google_functiongemma-270m-it-Q8_0.gguf"
var modelHash = "f50fbac8552d090863d5fefa983d24ac1ca37df23b1c77e3bbbd80aeb3b208c4"

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
	sys.RunSudoCommand("rm", yupsPath)
}

func handleAC() {
	slog.Info("Straw-boss (AC Mode).")
	start := time.Now()
	const steps = 6

	sys.Step(1, steps, "Getting system info")
	info := sys.GetSystemInfo()
	sys.Step(2, steps, "Saving config file")
	saveConfigFile(info)

	sys.Step(3, steps, "Setting bash integration")
	if err := updateBashrc(true); err != nil {
		slog.Error("Failed to update .bashrc",
			"error", err)
		slog.Warn("Yups will work with limited functionality.")
	} else {
		slog.Info(".bashrc hooks updated successfully")
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		sys.Step(4, steps, "Installing 'provides' helper")
		installProvidesHelper()
		return nil
	})
	g.Go(func() error {
		sys.Step(5, steps, "Installing yups")
		copyExecutableToPath()
		return nil
	})
	g.Go(func() error {
		sys.Step(6, steps, "Downloading model")
		return downloadModel(ctx)
	})

	if err := g.Wait(); err != nil {
		slog.Error("Error config yups", "internal", err)
		os.Exit(1)
	}

	slog.Info("Yups configuration completed in ", "time", time.Since(start).Round(time.Second))
}

func saveConfigFile(info sys.Info) {
	viper.Set("os", info.OS)
	viper.Set("pm", info.PM)
	viper.Set("distro_id", info.DistroID)
	viper.Set("distro_version", info.DistroVersion)
	viper.Set("distro_pretty", info.DistroPretty)
	viper.Set("log_level", "info")

	if err := viper.WriteConfig(); err != nil {
		os.MkdirAll(filepath.Dir(viper.ConfigFileUsed()), 0755)
		err = viper.WriteConfig()

		if err != nil {
			slog.Error("Error writing config file", "file", viper.ConfigFileUsed(), "error", err)
		}
	}
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
			sys.RunSudoCommand("apt-get", "update")
			sys.RunSudoCommand("apt-get", "install", "-y", "apt-file")
			sys.RunSudoCommand("apt-file", "update")
		}
	case "pacman":
		if _, err := exec.LookPath("pkgfile"); err != nil {
			slog.Info("Installing pkgfile for advanced search...")
			sys.RunSudoCommand("pacman", "-S", "--noconfirm", "pkgfile")
			sys.RunSudoCommand("pkgfile", "--update")
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

	if err := sys.RunSudoCommand("cp", currentPath, targetPath); err != nil {
		slog.Error("Failed to copy executable to path", "error", err)
		return
	}

	sys.RunSudoCommand("chmod", "+x", targetPath)
}

func downloadModel(ctx context.Context) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".yups/models/gemma-3-270m.gguf")
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	resp, err := http.Get(modelUri)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, _ := os.Create(path + ".tmp")
	defer os.Remove(path + ".tmp")
	defer out.Close()

	counter := &sys.ProgressWriter{Total: uint64(resp.ContentLength), Message: "Downloading model"}
	_, _ = io.Copy(out, io.TeeReader(resp.Body, counter))

	fmt.Print("\n")

	os.Rename(path+".tmp", path)

	if !verifyChecksum(path, modelHash) {
		return sys.YupsError{Message: "Checksum verification failed"}
	}
	return nil
}

func verifyChecksum(path, expected string) bool {
	f, _ := os.Open(path)
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)) == expected
}
