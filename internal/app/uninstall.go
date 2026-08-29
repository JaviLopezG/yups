// uninstall.go - `yups --uninstall-yups`.
package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"yups/internal/config"
)

// Uninstall implements `yups --uninstall-yups`:
//
//  1. If the command is nowhere to be found (PATH plus the well-known
//     binary directories), the user is informed that it is already
//     uninstalled.
//  2. Every directory holding a yups executable is located. When there
//     are several, the user is asked whether to uninstall for all users;
//     declining keeps the system-wide copies and removes only the ones
//     inside the user home.
//  3. The executable is deleted from every target directory where the
//     user has permission, reporting what was removed.
//  4. If some copy could not be removed and the user belongs to an
//     administrator group, repeating the command with sudo is suggested.
//  5. Finally, the user is asked whether to keep configuration files and
//     request history (default: keep). If the user opts to delete them,
//     an administrator is asked whether to delete for all users or only
//     the current user.
func Uninstall(env *Env, stdout, stderr io.Writer) int {
	candidates := append(append([]string{}, env.PathDirs()...), env.KnownBinDirs()...)

	found := findInDirs(env, candidates, ProgramName)
	if len(found) == 0 {
		fmt.Fprintf(stdout, "%s is not installed (already uninstalled).\n", ProgramName)
		askToDeleteStateDir(env, stdout)
		return ExitOK
	}

	targets := found
	if len(found) > 1 {
		question := fmt.Sprintf("%s is installed in %d places (%s). Uninstall for all users?",
			ProgramName, len(found), quotedJoin(found))
		if !env.AskConfirmation(question, true) {
			home, err := env.UserHomeDir()
			if err != nil {
				fmt.Fprintf(stderr, "Cannot determine the home directory: %v.\n", err)
				return ExitError
			}
			targets = dirsInsideHome(found, home)
			if len(targets) == 0 {
				fmt.Fprintf(stdout, "Keeping the system-wide installations (%s).\n", quotedJoin(found))
				askToDeleteStateDir(env, stdout)
				return ExitOK
			}
		}
	}

	var removed, blocked []string
	for _, dir := range targets {
		target := filepath.Join(dir, ProgramName)
		switch err := env.Remove(target); {
		case err == nil:
			removed = append(removed, target)
		case errors.Is(err, fs.ErrNotExist) || !env.LookupExecutable(dir, ProgramName):
			// The file is already gone: this happens when the same inode
			// is reachable through several PATH directories (e.g. /bin
			// being a symlink of /usr/bin), so count it as removed.
			removed = append(removed, target)
		default:
			blocked = append(blocked, target)
		}
	}

	for _, path := range removed {
		fmt.Fprintf(stdout, "Removed %s.\n", path)
	}

	exitCode := ExitOK
	if len(blocked) > 0 {
		exitCode = ExitError
		fmt.Fprintf(stdout, "Could not remove %s.\n", strings.Join(blocked, ", "))
		if isAdmin(env) {
			fmt.Fprint(stdout, "You have administrator privileges: retry the previous command with sudo (sudo !!).\n")
		} else {
			fmt.Fprintf(stdout, "You do not have permissions to uninstall %s.\n", ProgramName)
		}
	}

	if home, err := env.UserHomeDir(); err == nil {
		if RemoveBashBinding(env, home) {
			fmt.Fprintf(stdout, "Removed YUPS shell integration from %s.\n", filepath.Join(home, ".bashrc"))
		}
	}

	askToDeleteStateDir(env, stdout)
	return exitCode
}

// askToDeleteStateDir offers keeping ~/.yups (configuration, logs and
// interaction history). Default is to keep. If declined, administrator users
// are asked whether to delete for all users or only the current user.
func askToDeleteStateDir(env *Env, stdout io.Writer) {
	home, err := env.UserHomeDir()
	if err != nil {
		return
	}
	stateDir := config.Dir(home)
	if !env.PathExists(stateDir) {
		return
	}
	if env.AskConfirmation("Keep configuration files and request history?", true) {
		fmt.Fprintf(stdout, "Keeping %s.\n", stateDir)
		return
	}

	if isAdmin(env) {
		if env.AskConfirmation("Delete configuration and history for all users?", false) {
			dirs := []string{stateDir}
			if home != "/root" && env.PathExists("/root/.yups") {
				dirs = append(dirs, "/root/.yups")
			}
			if matches, err := filepath.Glob("/home/*/.yups"); err == nil {
				for _, m := range matches {
					if env.PathExists(m) {
						dirs = append(dirs, m)
						RemoveBashBinding(env, filepath.Dir(m))
					}
				}
			}
			RemoveBashBinding(env, "/root")
			for _, dir := range dedupeDirs(dirs) {
				if err := env.RemoveAll(dir); err != nil {
					fmt.Fprintf(stdout, "Could not delete %s: %v.\n", dir, err)
				} else {
					fmt.Fprintf(stdout, "Deleted %s.\n", dir)
				}
			}
			return
		}
	}

	if err := env.RemoveAll(stateDir); err != nil {
		fmt.Fprintf(stdout, "Could not delete %s: %v.\n", stateDir, err)
		return
	}
	fmt.Fprintf(stdout, "Deleted %s.\n", stateDir)
}

// dirsInsideHome filters dirs down to the ones located inside home.
func dirsInsideHome(dirs []string, home string) []string {
	var out []string
	for _, dir := range dedupeDirs(dirs) {
		if dir == home || strings.HasPrefix(dir, home+"/") {
			out = append(out, dir)
		}
	}
	return out
}
