// migrate.go - versioned, idempotent migrations applied by phase 2 of the
// self-update, plus the ~/.yups/state.toml progress file.
//
// The registry is intentionally empty today: future releases append their
// steps here and every installation catches up automatically, even when
// jumping several versions at once (each step records its completion in
// state.toml right after running, so an interrupted update resumes where
// it stopped).
package app

import (
	"path/filepath"

	"yups/internal/config"
	"yups/internal/semver"
)

// Migration is one self-update step introduced by a specific release. It
// must be idempotent: a step can legitimately run again after a partial
// failure (for example when some installed copies could not be replaced
// and the user retries with sudo).
type Migration struct {
	// Version of the release that introduced the step.
	Version string
	// Name shown in reports and errors.
	Name string
	// Apply performs the step; home is the current user home directory.
	Apply func(env *Env, home string) error
}

// migrations lists every version-introduced step in ascending version
// order. Empty for now (approved design decision 7).
var migrations = []Migration{}

// StateFileName is the progress file living next to config.toml.
const StateFileName = "state.toml"

// statePath returns the migration progress file path under home.
func statePath(home string) string {
	return filepath.Join(config.Dir(home), StateFileName)
}

// RunMigrations applies every pending migration up to targetVersion and
// returns how many ran. Progress is recorded after each step, so re-running
// never repeats completed work.
func RunMigrations(env *Env, home, targetVersion string) (int, error) {
	stateFile := statePath(home)
	state, err := env.LoadUpdateState(stateFile)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, m := range migrations {
		if !migrationPending(state.LastApplied, m.Version, targetVersion) {
			continue
		}
		if err := m.Apply(env, home); err != nil {
			return applied, err
		}
		applied++
		state.LastApplied = m.Version
		if err := env.SaveUpdateState(stateFile, state); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

// migrationPending reports whether a step introduced in version must run:
// strictly newer than the last recorded one and not newer than the binary
// now running. A dev build (or any unparseable target) applies nothing,
// keeping development installs untouched.
func migrationPending(lastApplied, version, targetVersion string) bool {
	return semver.Compare(version, lastApplied) > 0 &&
		semver.Compare(version, targetVersion) <= 0
}
