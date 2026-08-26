// Package app implements the yups commands: printing the logo, --help,
// --install and --uninstall.
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
  yups                 Print the logo (` + Logo + `) in ANSI colour 214
  yups --help          Show this help text
  yups --version       Show the yups version
  yups --install       Install the yups executable into the first directory
                       of the PATH where the current user can write
  yups --uninstall     Remove every yups executable found in the PATH
  yups --update-yups   Download the latest released version and replace
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
	case "-i", "--install", "install":
		return Install(env, stdout, stderr)
	case "-u", "--uninstall", "uninstall":
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

// selectKeeper chooses which installation keeps operating when several
// copies of the executable exist (approved design decision 8):
//
//  1. directories outside /usr/... and /home/... win: they usually mean a
//     deliberate, user-managed location;
//  2. among ties, the directory holding the currently running executable;
//  3. otherwise the first result.
//
// It returns the keeper and every other location. v1.0 only informs about
// the duplicates; nothing is ever deleted automatically.
func selectKeeper(dirs []string, runningExecutable string) (keeper string, others []string) {
	all := dedupeDirs(dirs)

	candidates := all
	var outside []string
	for _, dir := range all {
		if !underSystemOrHome(dir) {
			outside = append(outside, dir)
		}
	}
	if len(outside) > 0 {
		candidates = outside
	}

	keeper = candidates[0]
	if runningExecutable != "" {
		runningDir := filepath.Dir(runningExecutable)
		for _, dir := range candidates {
			if dir == runningDir {
				keeper = dir
				break
			}
		}
	}

	// Every other location counts as a duplicate worth reporting, not
	// just the ones that took part in the keeper election.
	for _, dir := range all {
		if dir != keeper {
			others = append(others, dir)
		}
	}
	return keeper, others
}

// underSystemOrHome reports whether dir is /usr or /home themselves or
// hangs from either of them.
func underSystemOrHome(dir string) bool {
	for _, root := range []string{"/usr", "/home"} {
		if dir == root || strings.HasPrefix(dir, root+"/") {
			return true
		}
	}
	return false
}
