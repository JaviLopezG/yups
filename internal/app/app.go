// Package app implements the CLI entry point, flag dispatching,
// --install-yups and --uninstall-yups.
package app

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Version is the version of the binary. It is overridden at build time with
// -ldflags "-X yups/internal/app.Version=vX.Y.Z"; a plain `go build` leaves
// the development placeholder.
var Version = "dev"

// Process exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

const (
	// ProgramName is the name the executable is installed as.
	ProgramName = "yups"
	// Logo is the string printed by default.
	Logo = "#_?"
)

// ColoredLogo is Logo wrapped in ANSI 256-colour 214 (orange).
const ColoredLogo = "\x1b[38;5;214m" + Logo + "\x1b[0m"

const helpText = `yups - prints the ` + Logo + ` logo and manages its own installation

Usage:
  yups                  Print the logo (` + Logo + `) in ANSI colour 214
  yups --help           Show this help text
  yups --version        Show the yups version
  yups --install-yups   Install the yups executable into the first directory
                        of the PATH where the current user can write
  yups --uninstall-yups Remove every yups executable found in the PATH
  yups --update-yups    Download the latest released version and replace
                        every installed copy with it
`

// Dispatch parses args and runs the matching command, returning the exit
// code for the process.
func Dispatch(env *Env, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, ColoredLogo)
		return ExitOK
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, helpText)
		return ExitOK
	case "-V", "--version", "version":
		fmt.Fprintf(stdout, "%s %s\n", ProgramName, Version)
		return ExitOK
	case "-i", "--install-yups":
		return Install(env, stdout, stderr)
	case "-u", "--uninstall-yups":
		return Uninstall(env, stdout, stderr)
	case "--update-yups":
		return Update(env, stdout, stderr)
	case flagUpdateApply:
		return UpdateApply(env, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "yups: unknown option %q\n\n", args[0])
		fmt.Fprint(stderr, helpText)
		return ExitUsage
	}
}

// findInDirs returns the directories from dirs that contain an executable
// file called name, deduplicating references that point to the same physical
// binary (such as directory symlinks on merged-usr distributions).
func findInDirs(env *Env, dirs []string, name string) []string {
	var found []string
	seenFiles := make(map[string]bool)
	for _, dir := range dedupeDirs(dirs) {
		if env.LookupExecutable(dir, name) {
			fullPath := filepath.Join(dir, name)
			canonicalPath := fullPath
			if env.EvalSymlinks != nil {
				if realPath, err := env.EvalSymlinks(fullPath); err == nil && realPath != "" {
					canonicalPath = realPath
				}
			}
			if seenFiles[canonicalPath] {
				continue
			}
			seenFiles[canonicalPath] = true
			reportDir := dir
			if env.EvalSymlinks != nil {
				if realDir, err := env.EvalSymlinks(dir); err == nil && realDir == "/usr/bin" {
					reportDir = "/usr/bin"
				}
			}
			found = append(found, reportDir)
		}
	}
	return found
}

// dedupeDirs removes empty entries and duplicated directories while keeping
// the original order.
func dedupeDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}

// firstWritableDir returns the first directory of dirs where the current
// user can write, or "" when there is none.
func firstWritableDir(env *Env, dirs []string) string {
	for _, dir := range dedupeDirs(dirs) {
		if env.IsWritableDir(dir) {
			return dir
		}
	}
	return ""
}

// inSudoGroup reports whether any of the group names matches a well-known
// administrator group: sudo/sudoer/sudoers (Debian family and friends),
// wheel (Fedora, RHEL, Arch, openSUSE...) and admin (historic Ubuntu,
// macOS).
func inSudoGroup(groups []string) bool {
	for _, group := range groups {
		switch strings.ToLower(strings.TrimSpace(group)) {
		case "sudo", "sudoer", "sudoers", "wheel", "admin":
			return true
		}
	}
	return false
}

// isAdmin reports whether the current user can be expected to elevate
// privileges with sudo: either by belonging to one of the well-known
// administrator groups or by being able to run sudo without a password
// (root on single-user systems, cloud images, NOPASSWD sudoers entries).
func isAdmin(env *Env) bool {
	if groups, err := env.CurrentUserGroups(); err == nil && inSudoGroup(groups) {
		return true
	}
	return env.SudoWithoutPassword()
}

// quotedJoin renders each directory as its full executable path for user
// facing messages.
func quotedJoin(dirs []string) string {
	paths := make([]string, len(dirs))
	for i, dir := range dirs {
		paths[i] = filepath.Join(dir, ProgramName)
	}
	return strings.Join(paths, ", ")
}

// firstInPath selects the first executable found according to the PATH order
// (matching what `which` and shell invocation resolve). If none is in PATH,
// it picks the first discovered directory.
func firstInPath(env *Env, dirs []string) (keeper string, duplicates []string) {
	all := dedupeDirs(dirs)
	if len(all) == 0 {
		return "", nil
	}
	pathDirs := dedupeDirs(env.PathDirs())
	for _, p := range pathDirs {
		for _, d := range all {
			if d == p {
				keeper = d
				break
			}
		}
		if keeper != "" {
			break
		}
	}
	if keeper == "" {
		keeper = all[0]
	}
	for _, d := range all {
		if d != keeper {
			duplicates = append(duplicates, d)
		}
	}
	return keeper, duplicates
}
