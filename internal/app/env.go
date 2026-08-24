package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
}

// Run parses args and executes the matching command against the real
// operating system. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return Dispatch(NewOSEnv(), args, stdout, stderr)
}

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
// ACLs, read-only mounts or sticky bits.
func osIsWritableDir(dir string) bool {
	file, err := os.CreateTemp(dir, ".yups-writable-check-*")
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
