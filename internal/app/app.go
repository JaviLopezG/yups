// Package app implements the yups commands: printing the marker, --help,
// --install and --uninstall.
package app

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Process exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

const (
	// ProgramName is the name the executable is installed as.
	ProgramName = "yups"
	// Marker is the string printed by default.
	Marker = "#_?"
)

// ColoredMarker is Marker wrapped in ANSI 256-colour 214 (orange).
const ColoredMarker = "\x1b[38;5;214m" + Marker + "\x1b[0m"

const helpText = `yups - prints the ` + Marker + ` marker and manages its own installation

Usage:
  yups                 Print the marker (` + Marker + `) in ANSI colour 214
  yups --help          Show this help text
  yups --install       Install the yups executable into the first directory
                       of the PATH where the current user can write
  yups --uninstall     Remove every yups executable found in the PATH
`

// Dispatch parses args and runs the matching command, returning the exit
// code for the process.
func Dispatch(env *Env, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, ColoredMarker)
		return ExitOK
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, helpText)
		return ExitOK
	case "-i", "--install", "install":
		return Install(env, stdout, stderr)
	case "-u", "--uninstall", "uninstall":
		return Uninstall(env, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "yups: unknown option %q\n\n", args[0])
		fmt.Fprint(stderr, helpText)
		return ExitUsage
	}
}

// findInDirs returns the directories from dirs that contain an executable
// file called name.
func findInDirs(env *Env, dirs []string, name string) []string {
	var found []string
	for _, dir := range dedupeDirs(dirs) {
		if env.LookupExecutable(dir, name) {
			found = append(found, dir)
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
