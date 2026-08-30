// update.go - the two-phase self-update (approved design decision 6).
//
// Phase 1 (`yups --update-yups`) runs in the OLD binary: it loads the
// configuration, resolves the latest release (Forgejo canonical, GitHub
// fallback), downloads and verifies the archive, stages and self-validates
// the new binary, then hands control over with syscall.Exec.
//
// Phase 2 (`yups --update-apply`) runs in the NEW binary: only it knows
// how the system should look after this version. It replaces every
// installed copy atomically, bumps config.version forward, runs pending
// migrations and cleans the staging directory.
package app

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"yups/internal/config"
	"yups/internal/semver"
	"yups/internal/sessionlog"
	"yups/internal/update"
)

// flagUpdateApply is a system flag: users are not expected to type it,
// phase 1 execs the freshly downloaded binary with it.
const flagUpdateApply = "--update-apply"

// Update implements `yups --update-yups`, the fetcher phase.
func Update(env *Env, stdout, stderr io.Writer) int {
	home, err := env.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "Cannot update %s: %v.\n", ProgramName, err)
		return ExitError
	}

	// First run on this machine (acceptance criterion MU-1): warn about
	// the limited functionality and offer the per-user installation
	// before touching anything else. The update itself continues either
	// way with default values.
	stateDir := config.Dir(home)
	if !env.PathExists(stateDir) {
		fmt.Fprintf(stdout,
			"Warning: %s is not initialized (%s is missing); running with default values and limited functionality.\n",
			ProgramName, stateDir)
		if env.AskConfirmation("Do you want to run the per-user installation now?", false) {
			if code := Install(env, stdout, stderr); code != ExitOK {
				return code
			}
		}
	}

	cfgPath := config.Path(home)
	cfg, err := env.LoadConfig(cfgPath)
	if err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "cannot load config from %s: %v", cfgPath, err)
		fmt.Fprintf(stderr, "Cannot update %s: %v.\nFix or remove %s and retry.\n", ProgramName, err, cfgPath)
		return ExitError
	}

	release, err := latestRelease(cfg, env.HTTPClient(), stdout)
	if err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "cannot check for updates: %v", err)
		fmt.Fprintf(stderr, "Cannot check for updates: %v.\n", err)
		return ExitError
	}

	cmp := semver.Compare(release.Tag, Version)
	if cmp == 0 {
		fmt.Fprintf(stdout, "%s %s is up to date (latest release: %s).\n", ProgramName, Version, release.Tag)
		if env.DownloadCheatsheets != nil && env.HTTPClient != nil {
			if home, err := env.UserHomeDir(); err == nil {
				cheatsDir := config.CheatsheetsDir(home)
				_ = env.DownloadCheatsheets(env.HTTPClient(), cheatsDir, stdout)
			}
		}
		return ExitOK
	}
	if cmp < 0 {
		fmt.Fprintf(stdout, "The repository version (%s) is older than the current version (%s).\n", release.Tag, Version)
		if !env.AskConfirmation(fmt.Sprintf("Do you want to downgrade to %s?", release.Tag), false) {
			fmt.Fprintln(stdout, "Downgrade cancelled.")
			return ExitOK
		}
	}

	fmt.Fprintf(stdout, "Downloading %s...\n", release.Tag)
	archiveData, err := update.Fetch(env.HTTPClient(), release.AssetURL)
	if err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "cannot download %s: %v", update.ArchiveName(release.Tag), err)
		fmt.Fprintf(stderr, "Cannot download %s: %v.\n", update.ArchiveName(release.Tag), err)
		return ExitError
	}
	checksums, err := update.Fetch(env.HTTPClient(), release.ChecksumURL)
	if err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "cannot download %s: %v", update.ChecksumsFileName, err)
		fmt.Fprintf(stderr, "Cannot download %s: %v.\n", update.ChecksumsFileName, err)
		return ExitError
	}
	if err := update.VerifyChecksums(checksums, update.ArchiveName(release.Tag), archiveData); err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_CHECKSUM_ERROR", "checksum verification failed for %s: %v", release.Tag, err)
		fmt.Fprintf(stderr, "Aborting update: %v.\n", err)
		return ExitError
	}

	binaryPath, err := env.StageBinary(archiveData, release.Tag)
	if err != nil {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "staging binary failed for %s: %v", release.Tag, err)
		fmt.Fprintf(stderr, "Aborting update: %v.\n", err)
		return ExitError
	}

	candidates := append(append([]string{}, env.PathDirs()...), env.KnownBinDirs()...)
	installed := findInDirs(env, candidates, ProgramName)
	if len(installed) == 0 {
		// Best-effort cleanup: the primary message below is what matters.
		cleanupStaging(env, filepath.Dir(binaryPath))
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_NOT_INSTALLED", "yups is not installed")
		fmt.Fprintf(stdout, "%s is not installed; run 'yups --install-yups' instead of updating.\n", ProgramName)
		return ExitError
	}

	// Multi-instance handling: the first instance found in PATH is updated;
	// duplicates are reported and left untouched.
	keeper, duplicates := firstInPath(env, installed)
	if len(duplicates) > 0 {
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_MULTIPLE_BINARIES", "several copies found (%s)", quotedJoin(duplicates))
		fmt.Fprintf(stdout,
			"Warning: %s is installed in several places (%s); only %s will be updated, duplicates are left untouched.\n",
			ProgramName, quotedJoin(duplicates), filepath.Join(keeper, ProgramName))
	}

	stagingDir := filepath.Dir(binaryPath)
	fmt.Fprintf(stdout, "Applying %s (replacing %s)...\n", release.Tag, filepath.Join(keeper, ProgramName))
	argv := []string{ProgramName, flagUpdateApply, "--from", stagingDir, "--installed", keeper}
	if err := env.ExecSelf(binaryPath, argv); err != nil {
		// Best-effort cleanup: the exec failure is the error to report.
		cleanupStaging(env, stagingDir)
		sessionlog.RecordIncident(home, "", "yups --update-yups", "UPDATE_ERROR", "cannot exec %s: %v", binaryPath, err)
		fmt.Fprintf(stderr, "Cannot hand over to the new binary: %v.\n", err)
		return ExitError
	}
	return ExitOK // never reached: ExecSelf replaced the process image
}

// latestRelease resolves the newest published release: the primary
// repository first (Forgejo, the canonical source of truth) and the
// fallback second (approved design decisions 3 and 5).
func latestRelease(cfg config.Config, client *http.Client, stdout io.Writer) (update.Release, error) {
	release, primaryErr := queryLatest(cfg.YUPSRepo, client)
	if primaryErr == nil {
		return release, nil
	}
	fmt.Fprintf(stdout, "%s is not reachable (%v); trying fallback %s...\n",
		cfg.YUPSRepo, primaryErr, cfg.YUPSRepoFallback)

	release, fallbackErr := queryLatest(cfg.YUPSRepoFallback, client)
	if fallbackErr != nil {
		return update.Release{}, fmt.Errorf("primary source failed (%v) and fallback failed too (%v)", primaryErr, fallbackErr)
	}
	return release, nil
}

func queryLatest(repoURL string, client *http.Client) (update.Release, error) {
	source, err := update.NewSource(repoURL, client)
	if err != nil {
		return update.Release{}, err
	}
	return source.Latest()
}

// UpdateApply implements `yups --update-apply`, the applier phase running
// inside the NEW binary. Its exit code becomes the exit code of the whole
// update because phase 1 exec'd into it.
func UpdateApply(env *Env, args []string, stdout, stderr io.Writer) int {
	from, installedArg, err := parseUpdateApplyArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "yups: %v\n\n", err)
		fmt.Fprint(stderr, helpText)
		return ExitUsage
	}

	stagedBinary := filepath.Join(from, ProgramName)

	// Re-scan even though phase 1 reported the locations, and operate on
	// the union of both views: cheap insurance against anything that
	// changed between the two phases. The firstInPath rule picks the first
	// copy in PATH that gets replaced; duplicates stay informed but untouched.
	candidates := append(append([]string{}, env.PathDirs()...), env.KnownBinDirs()...)
	locations := dedupeDirs(append(splitCommaList(installedArg), findInDirs(env, candidates, ProgramName)...))
	if len(locations) == 0 {
		// Best-effort cleanup: the primary message below is what matters.
		cleanupStaging(env, from)
		fmt.Fprintf(stderr, "No installed copy of %s was found to replace; staging cleaned.\n", ProgramName)
		return ExitError
	}

	keeper, duplicates := firstInPath(env, locations)
	if len(duplicates) > 0 {
		fmt.Fprintf(stdout,
			"Warning: %s is present in several places (%s); only %s is updated, duplicates are left untouched.\n",
			ProgramName, quotedJoin(duplicates), filepath.Join(keeper, ProgramName))
	}

	var updated, blocked []string
	for _, dir := range []string{keeper} {
		if _, err := env.ReplaceExecutable(stagedBinary, dir); err != nil {
			blocked = append(blocked, filepath.Join(dir, ProgramName))
			continue
		}
		updated = append(updated, filepath.Join(dir, ProgramName))
	}
	for _, path := range updated {
		fmt.Fprintf(stdout, "Updated %s.\n", path)
	}

	exitCode := ExitOK
	if len(blocked) > 0 {
		exitCode = ExitError
		if home, err := env.UserHomeDir(); err == nil {
			sessionlog.RecordIncident(home, "", "yups --update-apply", "UPDATE_PERMISSION_DENIED", "could not update %s", strings.Join(blocked, ", "))
		}
		fmt.Fprintf(stdout, "Could not update %s.\n", strings.Join(blocked, ", "))
		if isAdmin(env) {
			fmt.Fprint(stdout, "You have administrator privileges: retry the previous command with sudo (sudo !!).\n")
		} else {
			fmt.Fprintf(stdout, "You do not have permissions to update those copies of %s.\n", ProgramName)
		}
		// The staging directory is kept on purpose: sudo !! re-runs this
		// exact command line, which still points at the staged binary.
	}

	// Record progress before anything else can fail: config.version is
	// updated to the newly running version. A corrupt config is reported
	// but does not abort: the binaries are already replaced; migrations
	// track themselves in state.toml independently.
	home, homeErr := env.UserHomeDir()
	if homeErr != nil {
		fmt.Fprintf(stdout, "Warning: cannot locate the home directory (%v); config.version and migrations skipped.\n", homeErr)
	} else if err := recordUpdateProgress(env, home, stdout); err != nil {
		fmt.Fprintf(stdout, "Warning: %v.\n", err)
	}

	if homeErr == nil {
		if env.DownloadCheatsheets != nil && env.HTTPClient != nil {
			cheatsDir := config.CheatsheetsDir(home)
			_ = env.DownloadCheatsheets(env.HTTPClient(), cheatsDir, stdout)
		}
		applied, err := RunMigrations(env, home, Version)
		if err != nil {
			fmt.Fprintf(stdout, "Warning: migration failed after updating: %v.\n", err)
			exitCode = ExitError
		} else if applied > 0 {
			fmt.Fprintf(stdout, "Applied %d migration(s).\n", applied)
		}
		EnsureBashBindingUpdated(env, home, stdout, stderr)
		_ = EnsureScriptsUpdated(env, home)
	}

	if len(blocked) == 0 {
		// Best-effort cleanup: safely delete only ephemeral .kk staging dirs.
		cleanupStaging(env, from)
	}
	if exitCode == ExitOK {
		fmt.Fprintf(stdout, "%s updated to %s.\n", ProgramName, Version)
	}
	return exitCode
}

// recordUpdateProgress sets config.version to the newly installed version.
func recordUpdateProgress(env *Env, home string, stdout io.Writer) error {
	cfgPath := config.Path(home)
	cfg, err := env.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("cannot read %s (%v); config.version not updated", cfgPath, err)
	}
	if !config.SetVersion(&cfg, Version) {
		return nil
	}
	if err := env.SaveConfig(cfgPath, cfg); err != nil {
		return fmt.Errorf("cannot write %s: %w", cfgPath, err)
	}
	return nil
}

// isSafeStagingDir ensures that a path passed to --from is strictly a disposable
// staging directory created by yups (prefixed with "yups-" and ending with ".kk"),
// avoiding catastrophic deletion of system or user directories like ~/.local/bin.
func isSafeStagingDir(dir string) bool {
	clean := filepath.Clean(dir)
	base := filepath.Base(clean)
	if clean == "/" || clean == "." || clean == "" {
		return false
	}
	if strings.HasPrefix(clean, "/bin") || strings.HasPrefix(clean, "/usr") ||
		strings.HasPrefix(clean, "/etc") || strings.HasPrefix(clean, "/var") ||
		strings.HasPrefix(clean, "/home") {
		return strings.HasPrefix(base, "yups-") && strings.HasSuffix(base, ".kk")
	}
	return strings.HasPrefix(base, "yups-") && strings.HasSuffix(base, ".kk")
}

func cleanupStaging(env *Env, dir string) {
	if isSafeStagingDir(dir) {
		_ = env.RemoveAll(dir)
	}
}

// parseUpdateApplyArgs reads the system flags of phase 2.
func parseUpdateApplyArgs(args []string) (from, installed string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--from needs the staging directory")
			}
			from = args[i+1]
			i++
		case "--installed":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--installed needs a comma separated directory list")
			}
			installed = args[i+1]
			i++
		default:
			return "", "", fmt.Errorf("unknown option %q for %s", args[i], flagUpdateApply)
		}
	}
	if from == "" {
		return "", "", fmt.Errorf("%s requires --from <staging dir>", flagUpdateApply)
	}
	return from, installed, nil
}

// splitCommaList splits a comma separated list, dropping empty entries.
func splitCommaList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
