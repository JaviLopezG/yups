package app

import (
	"fmt"
	"io"
)

// Install implements `yups --install`:
//
//  1. If the executable is already reachable through one of the PATH
//     directories, the user is informed that it is already installed.
//  2. If a command with the same name exists in one of the well-known
//     system binary directories, the user is informed that it is already
//     installed.
//  3. If the user cannot write in any of the PATH directories: members of
//     an administrator group (sudo, sudoer, sudoers) are suggested to
//     repeat the previous command with sudo (`sudo !!`); anybody else is
//     informed that the installation is not possible.
//  4. Otherwise, the executable is copied into the first writable PATH
//     directory and the user is informed.
func Install(env *Env, stdout, stderr io.Writer) int {
	pathDirs := env.PathDirs()

	if found := findInDirs(env, pathDirs, ProgramName); len(found) > 0 {
		fmt.Fprintf(stdout, "%s is already installed in %s.\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

	if found := findInDirs(env, env.KnownBinDirs(), ProgramName); len(found) > 0 {
		fmt.Fprintf(stdout, "%s is already installed (found outside PATH in %s).\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

	destDir := firstWritableDir(env, pathDirs)
	if destDir == "" {
		if isAdmin(env) {
			fmt.Fprintf(stdout,
				"Cannot install %s: none of the PATH directories is writable, but you have administrator privileges; retry the previous command with sudo (sudo !!).\n",
				ProgramName)
		} else {
			fmt.Fprintf(stdout,
				"Cannot install %s: you do not have write permissions on any of the directories where the system stores executables.\n",
				ProgramName)
		}
		return ExitError
	}

	sourcePath, err := env.ExecutablePath()
	if err != nil {
		fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, err)
		return ExitError
	}

	destPath, err := env.InstallTo(sourcePath, destDir)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, err)
		return ExitError
	}

	fmt.Fprintf(stdout, "%s installed in %s.\n", ProgramName, destPath)
	return ExitOK
}
