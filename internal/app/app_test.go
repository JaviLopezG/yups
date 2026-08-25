package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"
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
}

func newFakeFS() *fakeFS {
	return &fakeFS{
		files:    map[string]bool{},
		writable: map[string]bool{},
		blocked:  map[string]bool{},
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
	}
}

func runDispatch(t *testing.T, env *Env, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Dispatch(env, args, &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func TestNoArgumentsPrintsColoredMarker(t *testing.T) {
	out, code := runDispatch(t, newFakeFS().env())
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
	for _, want := range []string{"--help", "--version", "--install", "--uninstall", Logo} {
		if !strings.Contains(out, want) {
			t.Errorf("help output %q does not contain %q", out, want)
		}
	}
}

func TestVersionFlagPrintsNameAndVersion(t *testing.T) {
	for _, arg := range []string{"--version", "-V", "version"} {
		out, code := runDispatch(t, newFakeFS().env(), arg)
		if code != ExitOK {
			t.Fatalf("arg %q: exit code = %d, want %d", arg, code, ExitOK)
		}
		if want := ProgramName + " " + Version + "\n"; out != want {
			t.Errorf("arg %q: output = %q, want %q", arg, out, want)
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
	out, code := runDispatch(t, fs.env(), "--install")
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
	out, code := runDispatch(t, fs.env(), "--install")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if !strings.Contains(out, "already installed") || !strings.Contains(out, "/usr/local/sbin/yups") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestInstallCopiesIntoFirstWritablePATHDir(t *testing.T) {
	fs := newFakeFS()
	fs.writable["/usr/local/bin"] = true
	out, code := runDispatch(t, fs.env(), "--install")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitOK, out)
	}
	if fs.installedDst != "/usr/local/bin/yups" {
		t.Errorf("installed to %q, want /usr/local/bin/yups", fs.installedDst)
	}
	if !strings.Contains(out, "/usr/local/bin/yups") {
		t.Errorf("user was not informed of the destination: %q", out)
	}
}

func TestInstallWithoutPermissionsAndWithoutSudoFails(t *testing.T) {
	fs := newFakeFS() // nothing writable, no groups
	out, code := runDispatch(t, fs.env(), "--install")
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
		out, code := runDispatch(t, fs.env(), "--install")
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
	out, code := runDispatch(t, fs.env(), "--install")
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
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
	out, code := runDispatch(t, fs.env(), "--install")
	if code != ExitError {
		t.Fatalf("exit code = %d, want %d (%s)", code, ExitError, out)
	}
	if fs.installedDst != "" {
		t.Error("nothing should have been installed")
	}
}

func TestUninstallWhenNotInstalled(t *testing.T) {
	fs := newFakeFS()
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
	out, code := runDispatch(t, fs.env(), "--uninstall")
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
