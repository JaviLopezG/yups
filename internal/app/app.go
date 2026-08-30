// Package app implements the CLI entry point, flag dispatching,
// --install-yups and --uninstall-yups.
package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"yups/internal/config"
	"yups/internal/explain"
	"yups/internal/sessionlog"
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
  yups -- <command...>  Explain the given command line, flags, and operators
  yups <command...>     Explain the given command line
  yups --model <tag>    Use a specific Ollama model for inference
  yups --advanced       Use the advanced reasoning model directly
  yups --test-models    Run latency benchmark test on all installed models
  yups --help           Show this help text
  yups --version        Show the yups version
  yups --install-yups   Install the yups executable into the first directory
                        of the PATH where the current user can write
  yups --uninstall-yups Remove every yups executable found in the PATH
  yups --update-yups    Download the latest released version and replace
                        every installed copy with it
`

func isSystemInstalled(env *Env) bool {
	if env == nil {
		return false
	}
	if env.IsInstalled != nil {
		return env.IsInstalled()
	}
	inPath := false
	if env.PathDirs != nil && env.LookupExecutable != nil {
		for _, dir := range env.PathDirs() {
			if env.LookupExecutable(dir, ProgramName) {
				inPath = true
				break
			}
		}
	}
	configFileExists := false
	if env.UserHomeDir != nil && env.PathExists != nil {
		if home, err := env.UserHomeDir(); err == nil {
			configFileExists = env.PathExists(config.Dir(home)) || env.PathExists(config.Path(home))
		}
	}
	return inPath && configFileExists
}

// Dispatch parses args and runs the matching command, returning the exit
// code for the process.
func Dispatch(env *Env, args []string, stdout, stderr io.Writer) int {
	homeDir := ""
	if env.UserHomeDir != nil {
		homeDir, _ = env.UserHomeDir()
	}
	logger := sessionlog.New(homeDir, args)

	isInstalled := isSystemInstalled(env)

	if len(args) == 0 {
		fmt.Fprintln(stdout, ColoredLogo)
		if !isInstalled {
			fmt.Fprintln(stdout, "Note: yups is not installed or configured yet.")
			if env.AskConfirmation != nil && env.AskConfirmation("Do you want to start the automatic installation process now? (estimated time ~1-3 minutes)", true) {
				if logger != nil {
					logger.LogConclusion("", "", "", "LAUNCH_INSTALL", 0)
				}
				return Install(env, stdout, stderr)
			}
			fmt.Fprintln(stdout, "Run 'yups --install-yups' at any time to install.")
		}
		if logger != nil {
			logger.LogConclusion("", "", "", "LOGO", ExitOK)
		}
		return ExitOK
	}

	color := env.IsTerminalOutput != nil && env.IsTerminalOutput(stdout)

	// Handle flagUpdateApply
	if args[0] == flagUpdateApply {
		if logger != nil {
			logger.LogSection("UPDATE APPLY")
		}
		code := UpdateApply(env, args[1:], stdout, stderr)
		if logger != nil {
			logger.LogConclusion("", "", "", "UPDATE_APPLY", code)
		}
		return code
	}

	// Parse flags
	var overrideModel string
	var useAdvanced bool
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--test-models" {
			_, code := RunModelBenchmark(env, stdout, stderr, logger)
			return code
		}
		if arg == "-h" || arg == "--help" || arg == "help" {
			if logger != nil {
				logger.LogConclusion("", "", "", "HELP", ExitOK)
			}
			fmt.Fprint(stdout, helpText)
			return ExitOK
		}
		if arg == "-V" || arg == "--version" || arg == "version" {
			if logger != nil {
				logger.LogConclusion("", "", "", "VERSION", ExitOK)
			}
			fmt.Fprintf(stdout, "%s %s\n", ProgramName, Version)
			return ExitOK
		}
		if arg == "-i" || arg == "--install-yups" {
			if logger != nil {
				logger.LogSection("INSTALL")
			}
			code := Install(env, stdout, stderr)
			if logger != nil {
				logger.LogConclusion("", "", "", "INSTALL", code)
			}
			return code
		}
		if arg == "-u" || arg == "--uninstall-yups" {
			if logger != nil {
				logger.LogSection("UNINSTALL")
			}
			code := Uninstall(env, stdout, stderr)
			if _, err := env.UserHomeDir(); err == nil {
				logger.LogConclusion("", "", "", "UNINSTALL", code)
			}
			return code
		}
		if arg == "--update-yups" {
			if logger != nil {
				logger.LogSection("UPDATE")
			}
			code := Update(env, stdout, stderr)
			if logger != nil {
				logger.LogConclusion("", "", "", "UPDATE", code)
			}
			return code
		}
		if arg == "--advanced" {
			useAdvanced = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--model=") {
			overrideModel = strings.TrimPrefix(arg, "--model=")
			i++
			continue
		}
		if arg == "--model" {
			if i+1 < len(args) {
				overrideModel = args[i+1]
				i += 2
				continue
			}
			fmt.Fprintln(stderr, "yups: --model requires a model name argument")
			if logger != nil {
				logger.LogConclusion("", "", "", "MISSING_MODEL_ARG", ExitUsage)
			}
			return ExitUsage
		}
		if arg == "--query" {
			flagArgs := args[:i+1]
			invocationFlags := strings.Join(flagArgs, " ")
			queryArgs := args[i+1:]
			queryText := strings.TrimSpace(strings.Join(queryArgs, " "))
			if queryText == "" {
				fmt.Fprintln(stderr, "yups: --query requires a question or prompt argument")
				if logger != nil {
					logger.LogConclusion("", "", "", "MISSING_QUERY_ARG", ExitUsage)
				}
				return ExitUsage
			}
			docEnv := env.DocEnv()
			docEnv.InvocationFlags = invocationFlags
			docEnv.Logger = logger
			docEnv.UseAdvanced = true
			if overrideModel != "" {
				docEnv.OverrideModel = overrideModel
			}
			return explain.Explain(context.Background(), docEnv, []string{"# " + queryText}, stdout, stderr, color)
		}
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(stderr, "yups: unknown option %q\n\n", arg)
			fmt.Fprint(stderr, helpText)
			if logger != nil {
				logger.LogConclusion("", "", "", "UNKNOWN_OPTION", ExitUsage)
			}
			return ExitUsage
		}
		break
	}

	flagArgs := args[:i]
	var invocationFlags string
	if len(flagArgs) > 0 {
		invocationFlags = strings.Join(flagArgs, " ")
	}

	cmdArgs := args[i:]
	if len(cmdArgs) == 0 {
		fmt.Fprintln(stdout, ColoredLogo)
		if !isInstalled {
			fmt.Fprintln(stdout, "Note: yups is not installed or configured yet.")
			if env.AskConfirmation != nil && env.AskConfirmation("Do you want to start the automatic installation process now? (estimated time ~1-3 minutes)", true) {
				if logger != nil {
					logger.LogConclusion("", "", "", "LAUNCH_INSTALL", 0)
				}
				return Install(env, stdout, stderr)
			}
			fmt.Fprintln(stdout, "Run 'yups --install-yups' at any time to install.")
		}
		if logger != nil {
			logger.LogConclusion("", "", "", "LOGO", ExitOK)
		}
		return ExitOK
	}

	docEnv := env.DocEnv()
	docEnv.InvocationFlags = invocationFlags
	docEnv.Logger = logger
	docEnv.OverrideModel = overrideModel
	docEnv.UseAdvanced = useAdvanced

	return explain.Explain(context.Background(), docEnv, cmdArgs, stdout, stderr, color)
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
