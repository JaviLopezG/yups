package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"yups/internal/config"
	"yups/internal/llm"
)

// fakeFS is an in-memory Env for the unit tests.
type fakeFS struct {
	files       map[string]bool // absolute file path -> exists as executable
	writable    map[string]bool // directory -> writable by current user
	blocked     map[string]bool // absolute file path -> removal is not allowed
	groups      []string
	sudoNoPass  bool // sudo works without password
	execPathErr error

	installedDst string
	removed      []string

	// Self-update support: everything the update phases touch lives in
	// memory so no test ever touches the real filesystem or network.
	home           string
	configs        map[string]config.Config  // config file path -> contents
	corruptConfig  map[string]bool           // config paths that fail to parse
	httpResponses  map[string]*http.Response // URL -> canned response
	httpHits       []string                  // URLs actually requested
	stagedBinary   string                    // path returned by StageBinary
	stageErr       error
	removedAll     []string
	execCalls      []execCall
	replaceErrs    map[string]error // destination dir -> failure
	replacedDirs   []string
	stateLast      map[string]string // state file path -> last applied version
	corruptState   map[string]bool   // state paths that fail to parse
	existingPaths  map[string]bool   // paths reported as existing
	fileContents   map[string]string // path -> file string contents
	askScript      []bool            // scripted AskConfirmation answers
	askedQuestions []string          // every question with its default hint
}

// execCall records one ExecSelf invocation.
type execCall struct {
	path string
	argv []string
}

func newFakeFS() *fakeFS {
	return &fakeFS{
		files:         map[string]bool{},
		writable:      map[string]bool{},
		blocked:       map[string]bool{},
		home:          "/home/user",
		configs:       map[string]config.Config{},
		corruptConfig: map[string]bool{},
		httpResponses: map[string]*http.Response{},
		replaceErrs:   map[string]error{},
		stateLast:     map[string]string{},
		corruptState:  map[string]bool{},
		existingPaths: map[string]bool{},
		fileContents:  map[string]string{},
	}
}

func (f *fakeFS) addExecutable(path string) { f.files[path] = true }

func (f *fakeFS) env() *Env {
	return &Env{
		PathDirs: func() []string {
			return []string{"/home/user/bin", "/usr/local/bin", "/usr/bin"}
		},
		KnownBinDirs: func() []string {
			return []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}
		},
		LookupExecutable: func(dir, name string) bool {
			return f.files[dir+"/"+name]
		},
		IsWritableDir: func(dir string) bool {
			return f.writable[dir]
		},
		ExecutablePath: func() (string, error) {
			if f.execPathErr != nil {
				return "", f.execPathErr
			}
			return "/tmp/build/yups", nil
		},
		CurrentUserGroups: func() ([]string, error) {
			return f.groups, nil
		},
		SudoWithoutPassword: func() bool {
			return f.sudoNoPass
		},
		InstallTo: func(sourcePath, destDir string) (string, error) {
			f.installedDst = destDir + "/" + ProgramName
			return f.installedDst, nil
		},
		Remove: func(path string) error {
			if !f.files[path] || f.blocked[path] {
				return errors.New("permission denied")
			}
			delete(f.files, path)
			f.removed = append(f.removed, path)
			return nil
		},
		UserHomeDir: func() (string, error) {
			return f.home, nil
		},
		LoadConfig: func(path string) (config.Config, error) {
			if f.corruptConfig[path] {
				return config.Config{}, fmt.Errorf("corrupt configuration file %q", path)
			}
			if c, ok := f.configs[path]; ok {
				return c, nil
			}
			return config.Defaults(), nil
		},
		SaveConfig: func(path string, c config.Config) error {
			f.configs[path] = c
			return nil
		},
		LoadUpdateState: func(path string) (string, error) {
			if f.corruptState[path] {
				return "", fmt.Errorf("corrupt update state file %q", path)
			}
			return f.stateLast[path], nil
		},
		SaveUpdateState: func(path string, lastApplied string) error {
			f.stateLast[path] = lastApplied
			return nil
		},
		HTTPClient: func() *http.Client {
			return &http.Client{Transport: &fakeTransport{responses: f.httpResponses, hits: &f.httpHits}}
		},
		StageBinary: func(archive []byte, tag string) (string, error) {
			if f.stageErr != nil {
				return "", f.stageErr
			}
			return f.stagedBinary, nil
		},
		RemoveAll: func(path string) error {
			f.removedAll = append(f.removedAll, path)
			return nil
		},
		ExecSelf: func(path string, argv []string) error {
			f.execCalls = append(f.execCalls, execCall{path: path, argv: argv})
			return nil
		},
		ReplaceExecutable: func(sourcePath, destDir string) (string, error) {
			if err, ok := f.replaceErrs[destDir]; ok {
				return "", err
			}
			f.replacedDirs = append(f.replacedDirs, destDir)
			return filepath.Join(destDir, ProgramName), nil
		},
		PathExists: func(path string) bool {
			if f.existingPaths[path] {
				return true
			}
			if _, ok := f.configs[path]; ok {
				return true
			}
			if path == config.Path(f.home) && f.existingPaths[config.Dir(f.home)] {
				return true
			}
			return false
		},
		EvalSymlinks: func(path string) (string, error) {
			return path, nil
		},
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			hint := "(y/N)"
			if defaultYes {
				hint = "(Y/n)"
			}
			f.askedQuestions = append(f.askedQuestions, prompt+" "+hint)
			if len(f.askScript) > 0 {
				answer := f.askScript[0]
				f.askScript = f.askScript[1:]
				return answer
			}
			return defaultYes
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return nil, nil
		},
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "", nil
		},
		ManPage: func(ctx context.Context, cmd string) (string, error) {
			return "", nil
		},
		TypeCmd: func(ctx context.Context, cmd string) (string, error) {
			return "", nil
		},
		Stat: func(path string) (fs.FileInfo, error) {
			return nil, fs.ErrNotExist
		},
		IsTerminalOutput: func(w io.Writer) bool {
			return false
		},
		ReadFile: func(path string) ([]byte, error) {
			if content, ok := f.fileContents[path]; ok {
				return []byte(content), nil
			}
			return nil, os.ErrNotExist
		},
		WriteFile: func(path string, data []byte, perm fs.FileMode) error {
			f.fileContents[path] = string(data)
			f.existingPaths[path] = true
			return nil
		},
	}
}

func runDispatch(t *testing.T, env *Env, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Dispatch(env, args, &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func TestNoArgumentsPrintsColoredMarker(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	out, code := runDispatch(t, fs.env())
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if want := "\x1b[38;5;214m#_?\x1b[0m\n"; out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestHelpListsAvailableCommands(t *testing.T) {
	out, code := runDispatch(t, newFakeFS().env(), "--help")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	for _, want := range []string{"--help", "--version", "--install-yups", "--uninstall-yups", "--update-yups", Logo} {
		if !strings.Contains(out, want) {
			t.Errorf("help output %q does not contain %q", out, want)
		}
	}
}

func TestVersionFlagPrintsNameAndVersion(t *testing.T) {
	for _, arg := range []string{"--version", "-V"} {
		out, code := runDispatch(t, newFakeFS().env(), arg)
		if code != ExitOK {
			t.Fatalf("arg %q: exit code = %d, want %d", arg, code, ExitOK)
		}
		if want := ProgramName + " " + Version + "\n"; out != want {
			t.Errorf("arg %q: output = %q, want %q", arg, out, want)
		}
	}
}

func TestDispatchDoubleDashAlonePrintsLogo(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	out, code := runDispatch(t, fs.env(), "--")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if want := "\x1b[38;5;214m#_?\x1b[0m\n"; out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestDispatchExplainsCommand(t *testing.T) {
	fs := newFakeFS()
	env := fs.env()
	env.Whatis = func(ctx context.Context, cmd string) (string, error) {
		if cmd == "ls" {
			return "ls (1) - list directory contents", nil
		}
		return "", nil
	}
	env.RunCmdTimeout = func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
		if name == "ls" {
			return []byte("  -l    use long format\n"), nil
		}
		return nil, nil
	}

	out, code := runDispatch(t, env, "--", "ls", "-l")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	for _, want := range []string{"#_?", "Found: ls", "ls (1) - list directory contents", "-l found:", "use long format"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchBareCommandExplains(t *testing.T) {
	fs := newFakeFS()
	env := fs.env()
	env.Whatis = func(ctx context.Context, cmd string) (string, error) {
		if cmd == "ls" {
			return "ls (1) - list directory contents", nil
		}
		return "", nil
	}

	out, code := runDispatch(t, env, "ls", "-l")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(out, "Found: ls") {
		t.Errorf("output does not contain 'Found: ls'\nFull output:\n%s", out)
	}
}

func TestDispatchUnknownFlagFallsBackToLLM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen3-coder:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Flag -javi does not exist for ls.\nSuggested command: ls -la",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	fs.configs[config.Path(fs.home)] = config.Config{
		Inference: config.InferenceConfig{
			Endpoint:     ts.URL,
			DefaultModel: "qwen3-coder:latest",
		},
	}

	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.Whatis = func(ctx context.Context, cmd string) (string, error) {
		if cmd == "ls" {
			return "ls (1) - list directory contents", nil
		}
		return "", nil
	}

	out, code := runDispatch(t, env, "--", "ls", "-javi")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}

	for _, want := range []string{
		"#_?",
		"Found: ls",
		"-j: No description found.",
		"Asking LLM",
		" at " + ts.URL,
		"LLM Explanation:",
		"Flag -javi does not exist",
		"Suggested command:",
		"ls -la",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestUnknownOptionIsAUsageError(t *testing.T) {
	out, code := runDispatch(t, newFakeFS().env(), "--bogus")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(out, "--bogus") {
		t.Errorf("output %q should mention the unknown option", out)
	}
}

func TestInstallWhenAlreadyInPath(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "already installed") || !strings.Contains(out, "/usr/bin/yups") {
		t.Errorf("unexpected output %q", out)
	}
	if fs.installedDst != "" {
		t.Error("nothing should have been copied")
	}
}

func TestInstallDetectsSameCommandOutsideCurrentPath(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/local/sbin/yups") // exists outside the fake PATH
	fs.existingPaths[config.Dir(fs.home)] = true
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "already installed") || !strings.Contains(out, "/usr/local/sbin/yups") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestInstallWhenBinaryInPATHButConfigDirMissing(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/bin/yups")
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if strings.Contains(out, "already installed") {
		t.Errorf("output %q should not report already installed when ~/.yups is missing", out)
	}
	if !strings.Contains(out, "yups installed in /usr/bin/yups") {
		t.Errorf("expected installation confirmation in /usr/bin/yups, got %q", out)
	}
	if fs.installedDst != "" {
		t.Error("binary already in PATH should not be re-copied")
	}
	cfgPath := config.Path(fs.home)
	if _, ok := fs.configs[cfgPath]; !ok {
		t.Fatalf("config file was not initialized at %s", cfgPath)
	}
}

func TestIsSystemInstalledBothConditions(t *testing.T) {
	tests := []struct {
		name       string
		inPath     bool
		hasConfig  bool
		wantResult bool
	}{
		{
			name:       "neither binary nor config exists",
			inPath:     false,
			hasConfig:  false,
			wantResult: false,
		},
		{
			name:       "binary in path but config missing",
			inPath:     true,
			hasConfig:  false,
			wantResult: false,
		},
		{
			name:       "config exists but binary missing from path",
			inPath:     false,
			hasConfig:  true,
			wantResult: false,
		},
		{
			name:       "both binary in path and config directory exist",
			inPath:     true,
			hasConfig:  true,
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFakeFS()
			if tt.inPath {
				fs.addExecutable("/usr/bin/yups")
			}
			if tt.hasConfig {
				fs.existingPaths[config.Dir(fs.home)] = true
			}
			got := isSystemInstalled(fs.env())
			if got != tt.wantResult {
				t.Errorf("isSystemInstalled = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestInstallCopiesIntoFirstWritablePATHDirAndInitializesConfig(t *testing.T) {
	fs := newFakeFS()
	fs.writable["/usr/local/bin"] = true
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if fs.installedDst != "/usr/local/bin/yups" {
		t.Errorf("installed to %q, want /usr/local/bin/yups", fs.installedDst)
	}
	if !strings.Contains(out, "/usr/local/bin/yups") {
		t.Errorf("user was not informed of the destination: %q", out)
	}
	cfgPath := config.Path(fs.home)
	cfg, ok := fs.configs[cfgPath]
	if !ok {
		t.Fatalf("config file was not initialized at %s", cfgPath)
	}
	if cfg.Version != Version {
		t.Errorf("initialized config version = %q, want %q", cfg.Version, Version)
	}
	if cfg.YUPSRepo != config.DefaultYUPSRepo {
		t.Errorf("initialized config YUPSRepo = %q, want %q", cfg.YUPSRepo, config.DefaultYUPSRepo)
	}
}

func TestInstallDownloadsCheatsheets(t *testing.T) {
	fs := newFakeFS()
	fs.writable["/usr/local/bin"] = true
	var downloadedTo string
	env := fs.env()
	env.DownloadCheatsheets = func(client *http.Client, destDir string, stdout io.Writer) error {
		downloadedTo = destDir
		fmt.Fprintln(stdout, "Downloaded mock cheatsheets")
		return nil
	}

	out, code := runDispatch(t, env, "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	wantDir := config.CheatsheetsDir(fs.home)
	if downloadedTo != wantDir {
		t.Errorf("downloadedTo = %q, want %q", downloadedTo, wantDir)
	}
	if !strings.Contains(out, "Downloaded mock cheatsheets") {
		t.Errorf("expected cheatsheet download message in output: %q", out)
	}
}

func TestInstallReviewConfigFileOpensEditor(t *testing.T) {
	fs := newFakeFS()
	fs.writable["/usr/local/bin"] = true
	var openedEditorPath string
	env := fs.env()
	env.AskConfirmation = func(prompt string, defaultYes bool) bool {
		if strings.Contains(prompt, "review the configuration file") {
			return true
		}
		return defaultYes
	}
	env.OpenEditor = func(path string, stdin io.Reader, stdout, stderr io.Writer) error {
		openedEditorPath = path
		return nil
	}

	out, code := runDispatch(t, env, "--install-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	wantPath := config.Path(fs.home)
	if openedEditorPath != wantPath {
		t.Errorf("openedEditorPath = %q, want %q", openedEditorPath, wantPath)
	}
	if !strings.Contains(out, "Configuration saved to") {
		t.Errorf("expected config location in output: %q", out)
	}
}

func TestInstallWithoutPermissionsAndWithoutSudoFails(t *testing.T) {
	fs := newFakeFS() // nothing writable, no groups
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if strings.Contains(out, "sudo !!") {
		t.Errorf("sudo must not be suggested without membership: %q", out)
	}
	if !strings.Contains(out, "write permissions") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestInstallWithoutPermissionsAndWithSudoSuggestsSudoBangBang(t *testing.T) {
	for _, group := range []string{"sudo", "sudoer", "sudoers", "wheel", "admin"} {
		fs := newFakeFS()
		fs.groups = []string{"users", group}
		out, code := runDispatch(t, fs.env(), "--install-yups")
		if code != ExitError {
			t.Fatalf("group %q: exit code = %d, want %d (%s)", group, code, ExitError, out)
		}
		if !strings.Contains(out, "sudo !!") {
			t.Errorf("group %q: expected sudo !! hint, got %q", group, out)
		}
	}
}

func TestInstallWithoutGroupButPasswordlessSudoSuggestsSudoBangBang(t *testing.T) {
	fs := newFakeFS()
	fs.sudoNoPass = true // e.g. root or a NOPASSWD sudoers entry
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "sudo !!") {
		t.Errorf("expected sudo !! hint, got %q", out)
	}
}

func TestUninstallBlockedWithPasswordlessSudoSuggestsSudoBangBang(t *testing.T) {
	fs := newFakeFS()
	fs.sudoNoPass = true
	fs.addExecutable("/usr/bin/yups")  // exists...
	fs.blocked["/usr/bin/yups"] = true // ...but cannot be removed
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "sudo !!") {
		t.Errorf("expected sudo !! hint, got %q", out)
	}
}

func TestUninstallWithWheelGroupSuggestsSudoBangBang(t *testing.T) {
	fs := newFakeFS()
	fs.groups = []string{"wheel"}
	fs.addExecutable("/usr/bin/yups")  // exists...
	fs.blocked["/usr/bin/yups"] = true // ...but cannot be removed
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "sudo !!") {
		t.Errorf("expected sudo !! hint, got %q", out)
	}
}

func TestInstallReportsExecutableResolutionFailure(t *testing.T) {
	fs := newFakeFS()
	fs.writable["/usr/bin"] = true
	fs.execPathErr = errors.New("cannot resolve")
	out, code := runDispatch(t, fs.env(), "--install-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if fs.installedDst != "" {
		t.Error("nothing should have been installed")
	}
}

func TestUninstallWhenNotInstalled(t *testing.T) {
	fs := newFakeFS()
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "already uninstalled") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestUninstallRemovesEveryCopy(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/home/user/bin/yups")
	fs.addExecutable("/usr/bin/yups")
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if len(fs.removed) != 2 {
		t.Fatalf("removed = %v, want both copies", fs.removed)
	}
	for _, want := range []string{"/home/user/bin/yups", "/usr/bin/yups"} {
		if !strings.Contains(out, "Removed "+want) {
			t.Errorf("output %q does not report removal of %s", out, want)
		}
	}
}

func TestUninstallBlockedWithoutSudoInformsUser(t *testing.T) {
	fs := newFakeFS()
	fs.addExecutable("/usr/bin/yups")  // exists...
	fs.blocked["/usr/bin/yups"] = true // ...but cannot be removed
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if strings.Contains(out, "sudo !!") {
		t.Errorf("sudo must not be suggested without membership: %q", out)
	}
	if !strings.Contains(out, "do not have permissions") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestUninstallBlockedWithSudoSuggestsSudoBangBang(t *testing.T) {
	fs := newFakeFS()
	fs.groups = []string{"sudo"}
	fs.addExecutable("/usr/bin/yups")  // exists...
	fs.blocked["/usr/bin/yups"] = true // ...but cannot be removed
	out, code := runDispatch(t, fs.env(), "--uninstall-yups")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if !strings.Contains(out, "sudo !!") {
		t.Errorf("expected sudo !! hint, got %q", out)
	}
}

func TestDedupeDirsDropsEmptyAndDuplicates(t *testing.T) {
	got := dedupeDirs([]string{"", "/a", "/a", "/b", ""})
	want := []string{"/a", "/b"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestOSCurrentUserGroupsReadsRealGroupsFile(t *testing.T) {
	groups, err := osCurrentUserGroups()
	if err != nil {
		t.Skipf("cannot resolve groups in this environment: %v", err)
	}
	if len(groups) == 0 {
		t.Fatal("expected at least the primary group")
	}
}

func TestOSAskConfirmationSharesStdinAcrossQuestions(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = reader
	resetReader := func() { stdinReader = sync.OnceValue(func() *bufio.Reader { return bufio.NewReader(os.Stdin) }) }
	resetReader()
	t.Cleanup(func() {
		os.Stdin = oldStdin
		resetReader()
		reader.Close()
		writer.Close()
	})

	if _, err := writer.WriteString("y\ny\n"); err != nil {
		t.Fatalf("writing answers: %v", err)
	}
	writer.Close()

	if !osAskConfirmation("first question?", true) {
		t.Error("first answer should be yes")
	}
	if !osAskConfirmation("second question?", false) {
		t.Error("second answer was lost: the stdin reader is not shared between questions")
	}
}

func TestDispatchFlagsHelpAndVersion(t *testing.T) {
	fs := newFakeFS()
	env := fs.env()

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--help"}, &stdout, &stderr)
	if code != ExitOK {
		t.Errorf("Dispatch(--help) = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout.String(), "--model") || !strings.Contains(stdout.String(), "--test-models") {
		t.Errorf("helpText missing new flags:\n%s", stdout.String())
	}
}

func TestDispatchTestModelsBenchmark(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			resp := map[string]any{
				"models": []map[string]any{
					{"name": "qwen2.5-coder:7b", "size": 4500000000},
					{"name": "gemma3:latest", "size": 6000000000},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "mock",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Sample response for benchmark",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.AskConfirmation = func(prompt string, defaultYes bool) bool { return true }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--test-models"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(--test-models) = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	for _, want := range []string{
		"Discovered 2 installed model(s)",
		"MODEL BENCHMARK SUMMARY",
		"qwen2.5-coder:7b",
		"gemma3:latest",
		"Response to verify acceptability:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchTestModelsBenchmarkColoredOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			resp := map[string]any{
				"models": []map[string]any{
					{"name": "qwen2.5-coder:7b", "size": 4500000000},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Flags explained.\nSuggested command: ls -la",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.IsTerminalOutput = func(w io.Writer) bool { return true }
	env.AskConfirmation = func(prompt string, defaultYes bool) bool { return true }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--test-models"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(--test-models) = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	for _, want := range []string{
		"\x1b[1;32m[PASSED]\x1b[0m",
		"\x1b[1;36mqwen2.5-coder:7b\x1b[0m",
		"Response to verify acceptability:",
		"Suggested command:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing colored segment %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchTestModelsBenchmarkIncludesToolCallingTime(t *testing.T) {
	var chatCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			resp := map[string]any{
				"models": []map[string]any{
					{"name": "qwen2.5-coder:7b", "size": 4500000000},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/chat" {
			chatCalls++
			if chatCalls == 1 {
				// Turn 1: tool call
				time.Sleep(10 * time.Millisecond)
				resp := llm.ChatResponse{
					Model: "qwen2.5-coder:7b",
					Message: llm.Message{
						Role: "assistant",
						ToolCalls: []llm.ToolCall{
							{
								Function: llm.ToolCallFunction{
									Name:      "fetch-command-documentation",
									Arguments: map[string]any{"command": "ls"},
								},
							},
						},
					},
					Done: true,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Turn 2: final answer
			time.Sleep(10 * time.Millisecond)
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Done after tool inspection.\nSuggested command: ls -la",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.AskConfirmation = func(prompt string, defaultYes bool) bool { return true }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--test-models"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(--test-models) = %d, want %d", code, ExitOK)
	}

	if chatCalls != 2 {
		t.Errorf("chatCalls = %d, want 2 (initial query + tool resolution)", chatCalls)
	}

	out := stdout.String()
	for _, want := range []string{
		"LLM requested detailed documentation for 'ls'",
		"MODEL BENCHMARK SUMMARY",
		"qwen2.5-coder:7b",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchQueryFlag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "gemma3:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Use df -h to see free disk space.\nSuggested command: df -h",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--query", "como", "ver", "el", "espacio", "en", "disco"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(--query) = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	for _, want := range []string{
		"# como ver el espacio en disco",
		"Asking advanced LLM",
		"Use df -h to see free disk space",
		"Suggested command:",
		"df -h",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchNaturalLanguageQuestionDirectly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "gemma3:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Use ip addr to see your IP address.\nSuggested command: ip addr",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"¿cual es mi ip?"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(question) = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	for _, want := range []string{
		"# ¿cual es mi ip?",
		"Asking advanced LLM",
		"Use ip addr to see your IP address",
		"Suggested command:",
		"ip addr",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDispatchWithFlagsAndSessionLogging(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen3.8:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Done.\nSuggested command: ls -hal",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	fs := newFakeFS()
	fs.addExecutable("/usr/local/bin/yups")
	fs.existingPaths[config.Dir(fs.home)] = true
	env := fs.env()
	env.HTTPClient = func() *http.Client { return ts.Client() }
	env.LoadConfig = func(path string) (config.Config, error) {
		cfg := config.Defaults()
		cfg.Inference.Endpoint = ts.URL
		return cfg, nil
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, []string{"--advanced", "--", "ls", "-hal"}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch(--advanced -- ls -hal) = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	if !strings.Contains(out, "#_? --advanced -- ls -hal") {
		t.Errorf("stdout missing flags in header '#_? --advanced -- ls -hal', got:\n%s", out)
	}
}

func TestOSReadHistory(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test live session history via YUPS_SESSION_HISTORY
	sessionHistFile := filepath.Join(tempDir, "session_history.kk")
	sessionContent := `  966  ls -javi # quiero diferenciar entre binarios y archivos de texto
  967  yups ls -javi # quiero diferenciar entre binarios y archivos de texto
  968  yups --uninstall-yups
  969  repos/yups/yups --install-yups
  970  cd
  971  find . -type f -exec file {} \; | grep -E "(ASCII text|Unicode text|empty)"
  972  ls #marca
  973  cat .bash_history
  974  history
  975  history|tail
`
	if err := os.WriteFile(sessionHistFile, []byte(sessionContent), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("YUPS_SESSION_HISTORY", sessionHistFile)

	lines := osReadHistory(tempDir, 5)
	if len(lines) != 5 {
		t.Fatalf("len(lines) = %d, want 5", len(lines))
	}
	if lines[len(lines)-1] != "cat .bash_history" {
		t.Errorf("last line = %q, want 'cat .bash_history'", lines[len(lines)-1])
	}
	if lines[len(lines)-2] != "ls #marca" {
		t.Errorf("second to last line = %q, want 'ls #marca'", lines[len(lines)-2])
	}

	// 2. Test fallback to .bash_history when YUPS_SESSION_HISTORY is unset
	t.Setenv("YUPS_SESSION_HISTORY", "")
	bashHistFile := filepath.Join(tempDir, ".bash_history")
	bashHistContent := "#1629837264\ngit status\n#1629837270\ngit diff\n"
	if err := os.WriteFile(bashHistFile, []byte(bashHistContent), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	fallbackLines := osReadHistory(tempDir, 10)
	if len(fallbackLines) != 2 {
		t.Fatalf("len(fallbackLines) = %d, want 2, got %v", len(fallbackLines), fallbackLines)
	}
	if fallbackLines[0] != "git status" || fallbackLines[1] != "git diff" {
		t.Errorf("fallbackLines = %v, want ['git status', 'git diff']", fallbackLines)
	}
}

func TestDispatchUninstalledOffersInstallation(t *testing.T) {
	fs := newFakeFS()
	// Clear installation state
	fs.files = map[string]bool{}
	fs.existingPaths = map[string]bool{}
	env := fs.env()
	env.AskConfirmation = func(question string, defaultVal bool) bool {
		return false
	}

	var stdout, stderr bytes.Buffer
	code := Dispatch(env, nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("Dispatch uninstalled = %d, want %d", code, ExitOK)
	}

	out := stdout.String()
	if !strings.Contains(out, "Note: yups is not installed or configured yet.") {
		t.Errorf("stdout missing uninstalled notice:\n%s", out)
	}
	if !strings.Contains(out, "Run 'yups --install-yups' at any time to install.") {
		t.Errorf("stdout missing install hint:\n%s", out)
	}
}
