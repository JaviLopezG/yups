package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// Uninstall implements `yups --uninstall`:
//
//  1. If the command is nowhere to be found (PATH plus the well-known
//     binary directories), the user is informed that it is already
//     uninstalled.
//  2. Every directory holding a yups executable is located.
//  3. The executable is deleted from every directory where the user has
//     permission, reporting what was removed.
//  4. If some copy could not be removed and the user belongs to an
//     administrator group, repeating the command with sudo is suggested.
//  5. If some copy could not be removed and the user has no administrator
//     privileges, the user is informed that the uninstall is not possible.
func Uninstall(env *Env, stdout, stderr io.Writer) int {
	candidates := append(append([]string{}, env.PathDirs()...), env.KnownBinDirs()...)

	found := findInDirs(env, candidates, ProgramName)
	if len(found) == 0 {
		fmt.Fprintf(stdout, "%s is not installed (already uninstalled).\n", ProgramName)
		return ExitOK
	}

	var removed, blocked []string
	for _, dir := range found {
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

	if len(blocked) == 0 {
		return ExitOK
	}

	fmt.Fprintf(stdout, "Could not remove %s.\n", strings.Join(blocked, ", "))
	if isAdmin(env) {
		fmt.Fprint(stdout, "You have administrator privileges: retry the previous command with sudo (sudo !!).\n")
	} else {
		fmt.Fprintf(stdout, "You do not have permissions to uninstall %s.\n", ProgramName)
	}
	return ExitError
}
