package app

import (
	"fmt"
	"io"
	"path/filepath"

	"yups/internal/config"
)

// Install implements `yups --install-yups`:
//
//  1. The write-permission probe over the PATH directories runs first,
//     before anything else (acceptance criterion IN-1).
//  2. If the executable is already reachable through one of the PATH
//     directories, the user is informed that it is already installed.
//  3. If a command with the same name exists in one of the well-known
//     system binary directories, the user is informed that it is already
//     installed.
//  4. When several coexisting copies show up anywhere, they are reported
//     together with the first instance found in PATH.
//  5. If the user cannot write in any of the PATH directories: members of
//     an administrator group (sudo, sudoer, sudoers) are suggested to
//     repeat the previous command with sudo (`sudo !!`); anybody else is
//     informed that the installation is not possible.
//  6. Otherwise, the executable is copied into the first writable PATH
//     directory, the ~/.yups configuration directory is initialized,
//     and the user is informed.
func Install(env *Env, stdout, stderr io.Writer) int {
	pathDirs := env.PathDirs()

	// Probed first so the answer is already in hand when it is needed;
	// the already-installed reports below stay correct even when the
	// user cannot write anywhere.
	destDir := firstWritableDir(env, pathDirs)

	if found := findInDirs(env, pathDirs, ProgramName); len(found) > 0 {
		reportInstallAnomaly(env, found, stdout)
		fmt.Fprintf(stdout, "%s is already installed in %s.\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

	if found := findInDirs(env, env.KnownBinDirs(), ProgramName); len(found) > 0 {
		reportInstallAnomaly(env, found, stdout)
		fmt.Fprintf(stdout, "%s is already installed (found outside PATH in %s).\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

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

	// Initialize ~/.yups/config.toml on install
	if home, err := env.UserHomeDir(); err == nil {
		cfgPath := config.Path(home)
		cfg, loadErr := env.LoadConfig(cfgPath)
		if loadErr != nil {
			cfg = config.Defaults()
		}
		config.EnsureDefaults(&cfg)
		if cfg.Version == config.FloorVersion || cfg.Version == "" {
			cfg.Version = Version
		}
		_ = env.SaveConfig(cfgPath, cfg)
	}

	fmt.Fprintf(stdout, "%s installed in %s.\n", ProgramName, destPath)
	return ExitOK
}

// reportInstallAnomaly warns about several coexisting installations and
// names the chosen target (first instance in PATH).
func reportInstallAnomaly(env *Env, found []string, stdout io.Writer) {
	if len(found) < 2 {
		return
	}
	keeper, others := firstInPath(env, found)
	fmt.Fprintf(stdout,
		"Warning: %s is installed in several places (%s); operating on %s.\n",
		ProgramName, quotedJoin(others), filepath.Join(keeper, ProgramName))
}
