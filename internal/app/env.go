package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"

	"yups/internal/config"
	"yups/internal/update"
)

// Env holds every interaction with the operating system that the commands
// need. Keeping them behind function fields makes the whole flow testable
// without touching a real filesystem or requiring root privileges.
type Env struct {
	// PathDirs returns the directories listed in the PATH variable.
	PathDirs func() []string
	// KnownBinDirs returns the well-known system directories where
	// executables live, regardless of the current PATH value.
	KnownBinDirs func() []string
	// LookupExecutable reports whether dir contains an executable file
	// with the given name.
	LookupExecutable func(dir, name string) bool
	// IsWritableDir reports whether the current user can create files in
	// dir.
	IsWritableDir func(dir string) bool
	// ExecutablePath returns the path of the running executable itself.
	ExecutablePath func() (string, error)
	// CurrentUserGroups returns the names of the groups of the current
	// user.
	CurrentUserGroups func() ([]string, error)
	// SudoWithoutPassword reports whether sudo can run without asking
	// for a password (NOPASSWD entries, root on single-user systems...).
	SudoWithoutPassword func() bool

	// InstallTo copies the running executable into destDir using
	// ProgramName as file name and returns the destination path.
	InstallTo func(sourcePath, destDir string) (string, error)
	// Remove deletes the file at path.
	Remove func(path string) error

	// UserHomeDir returns the home directory of the current user; the
	// ~/.yups directory hangs from it.
	UserHomeDir func() (string, error)
	// LoadConfig reads the configuration file: a missing file yields
	// defaults, a corrupt file is an explicit error (never silently
	// fall back to defaults in update paths).
	LoadConfig func(path string) (config.Config, error)
	// SaveConfig writes the configuration file, creating parent
	// directories as needed.
	SaveConfig func(path string, c config.Config) error
	// LoadUpdateState reads the last applied migration version from
	// ~/.yups/state.toml (empty when the file does not exist yet).
	LoadUpdateState func(path string) (string, error)
	// SaveUpdateState records the last applied migration version.
	SaveUpdateState func(path string, lastApplied string) error
	// HTTPClient returns the client used for release queries and asset
	// downloads.
	HTTPClient func() *http.Client
	// StageBinary extracts a downloaded release archive into a fresh
	// temporary staging directory, self-validates the extracted binary
	// (--version must report the expected tag) and returns the path of
	// the staged executable. The caller owns the staging directory and
	// removes it with RemoveAll.
	StageBinary func(archive []byte, tag string) (string, error)
	// RemoveAll deletes a directory tree (staging cleanup).
	RemoveAll func(path string) error
	// ExecSelf replaces the running process image with the program at
	// path; on success it never returns. The environment is inherited.
	ExecSelf func(path string, argv []string) error
	// ReplaceExecutable atomically installs sourcePath as ProgramName
	// inside destDir: the payload is copied to a temporary .kk file in
	// the destination directory, made executable and renamed onto the
	// final name, so readers never observe a half-written binary and a
	// cross-device rename can never happen.
	ReplaceExecutable func(sourcePath, destDir string) (string, error)

	// PathExists reports whether a filesystem path exists (files and
	// directories alike).
	PathExists func(path string) bool
	// AskConfirmation asks the user a yes/no question, returning
	// defaultYes when the input is empty, unavailable (piped scripts,
	// containers) or exhausted. It echoes the question so transcripts
	// stay readable.
	AskConfirmation func(prompt string, defaultYes bool) bool
}

// Run parses args and executes the matching command against the real
// operating system. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return Dispatch(NewOSEnv(), args, stdout, stderr)
}

// releaseHTTPClient is the shared client for release queries and asset
// downloads; the timeout keeps a stalled mirror from hanging the user's
// terminal forever.
var releaseHTTPClient = &http.Client{Timeout: 30 * time.Second}

// NewOSEnv returns an Env backed by the real operating system.
func NewOSEnv() *Env {
	return &Env{
		PathDirs:            osPathDirs,
		KnownBinDirs:        osKnownBinDirs,
		LookupExecutable:    osLookupExecutable,
		IsWritableDir:       osIsWritableDir,
		ExecutablePath:      os.Executable,
		CurrentUserGroups:   osCurrentUserGroups,
		SudoWithoutPassword: osSudoWithoutPassword,
		InstallTo:           osInstallTo,
		Remove:              os.Remove,
		UserHomeDir:         os.UserHomeDir,
		LoadConfig:          config.Load,
		SaveConfig:          config.Save,
		LoadUpdateState:     osLoadUpdateState,
		SaveUpdateState:     osSaveUpdateState,
		HTTPClient:          func() *http.Client { return releaseHTTPClient },
		StageBinary:         osStageBinary,
		RemoveAll:           os.RemoveAll,
		ExecSelf:            osExecSelf,
		ReplaceExecutable:   osReplaceExecutable,
		PathExists:          osPathExists,
		AskConfirmation:     osAskConfirmation,
	}
}

func osPathDirs() []string {
	return filepath.SplitList(os.Getenv("PATH"))
}

func osKnownBinDirs() []string {
	return []string{
		"/usr/local/sbin", "/usr/local/bin",
		"/usr/sbin", "/usr/bin",
		"/sbin", "/bin",
	}
}

func osLookupExecutable(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// osIsWritableDir probes the directory by actually creating (and removing)
// a temporary file in it: access(2)-style checks can be misleading with
// ACLs, read-only mounts or sticky bits. CreateTemp replaces the "*" with a
// random string (no collisions); the ".kk" extension marks the file as
// disposable junk, so anything left behind is trivially identifiable.
func osIsWritableDir(dir string) bool {
	file, err := os.CreateTemp(dir, ".yups-writable-check-*.kk")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

// osCurrentUserGroups resolves the group names of the current user,
// including the primary group.
func osCurrentUserGroups() ([]string, error) {
	current, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolving current user: %w", err)
	}
	ids, err := current.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("listing groups of %q: %w", current.Username, err)
	}

	names := make([]string, 0, len(ids)+1)
	lookup := func(id string) {
		if group, err := user.LookupGroupId(id); err == nil {
			names = append(names, group.Name)
		}
	}
	for _, id := range ids {
		lookup(id)
	}
	lookup(current.Gid)
	return names, nil
}

// osSudoWithoutPassword probes whether sudo can run without asking for a
// password: it is an extra signal beyond group membership, covering root on
// single-user systems, cloud images and NOPASSWD sudoers entries. When sudo
// is not even installed the probe simply reports false.
func osSudoWithoutPassword() bool {
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

func osInstallTo(sourcePath, destDir string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("opening %q: %w", sourcePath, err)
	}
	defer source.Close()

	destPath := filepath.Join(destDir, ProgramName)
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("creating %q: %w", destPath, err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return "", fmt.Errorf("copying %q to %q: %w", sourcePath, destPath, err)
	}
	return destPath, nil
}

// osStageBinary extracts a downloaded release archive into a fresh staging
// directory (the .kk suffix marks it as disposable junk), self-validates
// the extracted binary and returns its path. Any failure cleans the
// staging directory before returning.
func osStageBinary(archive []byte, tag string) (string, error) {
	staging, err := os.MkdirTemp("", "yups-update-*.kk")
	if err != nil {
		return "", fmt.Errorf("creating staging directory: %w", err)
	}

	if err := update.ExtractTarGz(archive, staging); err != nil {
		os.RemoveAll(staging)
		return "", err
	}

	binary := filepath.Join(staging, ProgramName)
	if err := update.ValidateBinary(binary, ProgramName, tag); err != nil {
		os.RemoveAll(staging)
		return "", err
	}
	return binary, nil
}

// osExecSelf replaces the running process image with the program at path;
// on success it never returns. The current environment is inherited so the
// next phase sees the same PATH/HOME the user has.
func osExecSelf(path string, argv []string) error {
	return syscall.Exec(path, argv, os.Environ())
}

// osReplaceExecutable installs sourcePath as ProgramName inside destDir
// atomically: temporary .kk file in the destination directory, chmod 0755,
// rename onto the final name. A same-directory rename is atomic and can
// never fail with EXDEV, unlike renaming across filesystems.
func osReplaceExecutable(sourcePath, destDir string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("opening %q: %w", sourcePath, err)
	}
	defer source.Close()

	tmp, err := os.CreateTemp(destDir, ".yups-update-*.kk")
	if err != nil {
		return "", fmt.Errorf("creating temporary file in %q: %w", destDir, err)
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, source); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("copying new %s into %q: %w", ProgramName, destDir, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("writing %q: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("making %q executable: %w", tmpName, err)
	}

	finalPath := filepath.Join(destDir, ProgramName)
	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("replacing %q: %w", finalPath, err)
	}
	return finalPath, nil
}

// osLoadUpdateState reads the last applied migration version from the
// state file; a missing file means nothing has been applied yet.
func osLoadUpdateState(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading update state %q: %w", path, err)
	}
	var state struct {
		LastApplied string `toml:"last-applied"`
	}
	if err := toml.Unmarshal(data, &state); err != nil {
		return "", fmt.Errorf("corrupt update state file %q: %w", path, err)
	}
	return state.LastApplied, nil
}

// osSaveUpdateState writes the state file, creating its parent directory
// when needed.
func osSaveUpdateState(path string, lastApplied string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", path, err)
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(struct {
		LastApplied string `toml:"last-applied"`
	}{LastApplied: lastApplied}); err != nil {
		return fmt.Errorf("encoding update state for %q: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing update state %q: %w", path, err)
	}
	return nil
}

// osPathExists reports whether the path exists.
func osPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// stdinReader is shared by every question of a run: creating a fresh
// bufio.Reader per question would silently swallow whatever the first
// reader buffered beyond the first answer.
var stdinReader = sync.OnceValue(func() *bufio.Reader {
	return bufio.NewReader(os.Stdin)
})

// osAskConfirmation reads a yes/no answer from stdin. An empty answer
// (plain Enter), EOF or three invalid answers fall back to the default,
// so piped or unattended invocations never hang on a question.
func osAskConfirmation(prompt string, defaultYes bool) bool {
	hint := "(y/N)"
	if defaultYes {
		hint = "(Y/n)"
	}

	reader := stdinReader()
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Printf("%s %s ", prompt, hint)
		line, err := reader.ReadString('\n')
		switch answer := strings.ToLower(strings.TrimSpace(line)); {
		case answer == "":
			return defaultYes
		case answer == "y", answer == "yes":
			return true
		case answer == "n", answer == "no":
			return false
		}
		if err != nil {
			return defaultYes
		}
		fmt.Println("Please answer y or n.")
	}
	return defaultYes
}
