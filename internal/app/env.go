package app

import (
	"bufio"
	"bytes"
	"context"
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
	"yups/internal/explain"
	"yups/internal/llm"
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
	// EvalSymlinks returns the path name after the evaluation of any symbolic
	// links.
	EvalSymlinks func(path string) (string, error)
	// AskConfirmation asks the user a yes/no question, returning
	// defaultYes when the input is empty, unavailable (piped scripts,
	// containers) or exhausted. It echoes the question so transcripts
	// stay readable.
	AskConfirmation func(prompt string, defaultYes bool) bool
	// AskPrompt prompts the user for text input with a default hint.
	AskPrompt func(prompt, defaultValue string) string

	// RunCmdTimeout runs name with args, bounded by the given timeout.
	RunCmdTimeout func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error)
	// Whatis queries whatis / apropos for cmd.
	Whatis func(ctx context.Context, cmd string) (string, error)
	// ManPage fetches the cat-rendered manual page for cmd.
	ManPage func(ctx context.Context, cmd string) (string, error)
	// TypeCmd inspects the shell type (alias, builtin) for cmd.
	TypeCmd func(ctx context.Context, cmd string) (string, error)
	// Stat returns file info for path.
	Stat func(path string) (fs.FileInfo, error)
	// IsTerminalOutput reports whether w is an interactive terminal.
	IsTerminalOutput func(w io.Writer) bool

	// Environment context gathering for LLM inference
	ReadOSRelease   func() string
	ReadHistory     func(home string, maxLines int) []string
	ReadFileSnippet func(path string, maxLines int) (string, error)
	ListDirNames    func(dir string, maxItems int) []string
	Getwd           func() (string, error)

	// ExecShell executes a command line in bash.
	ExecShell func(command string, stdout, stderr io.Writer) int
	// IsInstalled reports whether yups is installed on the system.
	IsInstalled func() bool
}

// DocEnv returns the explain.DocEnv adapter backed by this Env.
func (e *Env) DocEnv() explain.DocEnv {
	cfg := config.Defaults()
	isInstalled := false
	if e.IsInstalled != nil {
		isInstalled = e.IsInstalled()
	} else if e.UserHomeDir != nil && e.PathExists != nil {
		if home, err := e.UserHomeDir(); err == nil {
			isInstalled = e.PathExists(config.Path(home))
		}
	}

	if e.UserHomeDir != nil && e.LoadConfig != nil {
		if home, err := e.UserHomeDir(); err == nil {
			if loaded, err := e.LoadConfig(config.Path(home)); err == nil {
				cfg = loaded
			}
		}
	}
	config.EnsureDefaults(&cfg)

	var llmClient *llm.Client
	if cfg.InferenceEndpoint != "" && e.HTTPClient != nil {
		llmClient = llm.NewClient(e.HTTPClient(), cfg.InferenceEndpoint)
	}

	return explain.DocEnv{
		RunCmdTimeout: e.RunCmdTimeout,
		Whatis:        e.Whatis,
		ManPage:       e.ManPage,
		TypeCmd:       e.TypeCmd,
		StatPath:      e.Stat,
		LookupInPath: func(cmd string) bool {
			if e.PathDirs == nil || e.LookupExecutable == nil {
				return false
			}
			for _, dir := range e.PathDirs() {
				if e.LookupExecutable(dir, cmd) {
					return true
				}
			}
			return false
		},
		LLMClient:     llmClient,
		LLMEnv:        e.LLMEnv(),
		DefaultModel:  cfg.DefaultModel,
		AdvancedModel: cfg.AdvancedModel,
		IsInstalled:   isInstalled,
		AskPrompt:     e.AskPrompt,
		ExecShell:     e.ExecShell,
	}
}

// LLMEnv returns the llm.LLMEnv adapter backed by this Env.
func (e *Env) LLMEnv() llm.LLMEnv {
	return &envLLMAdapter{env: e}
}

type envLLMAdapter struct {
	env *Env
}

func (a *envLLMAdapter) UserHomeDir() (string, error) {
	if a.env.UserHomeDir == nil {
		return "", errors.New("no user home dir")
	}
	return a.env.UserHomeDir()
}

func (a *envLLMAdapter) Getwd() (string, error) {
	if a.env.Getwd == nil {
		return os.Getwd()
	}
	return a.env.Getwd()
}

func (a *envLLMAdapter) ReadOSRelease() string {
	if a.env.ReadOSRelease == nil {
		return ""
	}
	return a.env.ReadOSRelease()
}

func (a *envLLMAdapter) ReadHistory(home string, maxLines int) []string {
	if a.env.ReadHistory == nil {
		return nil
	}
	return a.env.ReadHistory(home, maxLines)
}

func (a *envLLMAdapter) ReadFileSnippet(path string, maxLines int) (string, error) {
	if a.env.ReadFileSnippet == nil {
		return "", errors.New("cannot read snippet")
	}
	return a.env.ReadFileSnippet(path, maxLines)
}

func (a *envLLMAdapter) ListDirNames(dir string, maxItems int) []string {
	if a.env.ListDirNames == nil {
		return nil
	}
	return a.env.ListDirNames(dir, maxItems)
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
		EvalSymlinks:        filepath.EvalSymlinks,
		AskConfirmation:     osAskConfirmation,
		AskPrompt:           osAskPrompt,
		RunCmdTimeout:       osRunCmdTimeout,
		Whatis:              osWhatis,
		ManPage:             osManPage,
		TypeCmd:             osTypeCmd,
		Stat:                os.Stat,
		IsTerminalOutput:    osIsTerminalOutput,
		ReadOSRelease:       osReadOSRelease,
		ReadHistory:         osReadHistory,
		ReadFileSnippet:     osReadFileSnippet,
		ListDirNames:        osListDirNames,
		Getwd:               os.Getwd,
		ExecShell:           osExecShell,
		IsInstalled:         osIsInstalled,
	}
}

func osPathDirs() []string {
	return filepath.SplitList(os.Getenv("PATH"))
}

func osKnownBinDirs() []string {
	return []string{
		"/usr/local/bin", "/usr/local/sbin",
		"/usr/bin", "/usr/sbin",
		"/bin", "/sbin",
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

// osRunCmdTimeout executes name with args and returns its output, bounded by timeout.
func osRunCmdTimeout(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// osWhatis queries whatis (and falls back to apropos -s 1,8) for cmd.
func osWhatis(ctx context.Context, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "whatis", cmd).CombinedOutput()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		out, err = exec.CommandContext(ctx, "apropos", "-s", "1,8", cmd).CombinedOutput()
	}
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// osManPage runs `man -P cat <cmd>` to fetch the plain text manual page.
func osManPage(ctx context.Context, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "man", "-P", "cat", cmd).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// osTypeCmd runs `bash -c "type <cmd>"` to discover aliases or builtins.
func osTypeCmd(ctx context.Context, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "bash", "-c", "type "+cmd).CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// osIsTerminalOutput reports whether w wraps an interactive character device terminal.
func osIsTerminalOutput(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			return true
		}
	}
	return false
}

// osAskPrompt prompts for user text input, returning defaultValue if empty.
func osAskPrompt(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	reader := stdinReader()
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

// osReadOSRelease parses /etc/os-release for human-readable distribution name.
func osReadOSRelease() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "PRETTY_NAME=") {
			val := strings.TrimPrefix(l, "PRETTY_NAME=")
			return strings.Trim(val, "\"")
		}
	}
	return "Linux"
}

// osReadHistory reads up to maxLines from shell history file.
func osReadHistory(home string, maxLines int) []string {
	if home == "" {
		return nil
	}
	histPath := filepath.Join(home, ".bash_history")
	data, err := os.ReadFile(histPath)
	if err != nil {
		histPath = filepath.Join(home, ".zsh_history")
		data, err = os.ReadFile(histPath)
	}
	if err != nil {
		return nil
	}
	rawLines := strings.Split(string(data), "\n")
	var valid []string
	for _, l := range rawLines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			valid = append(valid, trimmed)
		}
	}
	if len(valid) > maxLines {
		valid = valid[len(valid)-maxLines:]
	}
	return valid
}

// osReadFileSnippet reads up to maxLines from path if size is under 100KB.
func osReadFileSnippet(path string, maxLines int) (string, error) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Size() > 100*1024 {
		return "", errors.New("unsuitable for snippet")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lines []string
	count := 0
	for scanner.Scan() && count < maxLines {
		lines = append(lines, scanner.Text())
		count++
	}
	return strings.Join(lines, "\n"), nil
}

// osListDirNames lists up to maxItems entries in dir.
func osListDirNames(dir string, maxItems int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for i, e := range entries {
		if i >= maxItems {
			break
		}
		names = append(names, e.Name())
	}
	return names
}

// osExecShell executes command in a subshell, attaching standard IO streams.
func osExecShell(command string, stdout, stderr io.Writer) int {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return ExitError
	}
	return ExitOK
}

// osIsInstalled reports whether ~/.yups/config.toml exists on the system.
func osIsInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return osPathExists(config.Path(home))
}
