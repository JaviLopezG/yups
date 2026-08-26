// config.go - load, save and evolve ~/.yups/config.toml.
//
// The file tracks the highest version that has ever run on this install
// (so multi-version jumps can apply every intermediate migration) and the
// release repositories to query for self-updates. Missing file means
// defaults; a corrupt file is an explicit error: update code paths must
// never fall back to defaults silently, because that could re-run or skip
// migrations without anybody noticing.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"yups/internal/semver"
)

// Default repository URLs written when the configuration file does not
// exist. Forgejo is the canonical source of truth; GitHub is only a
// fallback (approved design decision 3).
const (
	DefaultYUPSRepo         = "https://code.javilopezg.com/javilopezg/yups"
	DefaultYUPSRepoFallback = "https://github.com/JaviLopezG/yups"
	// FloorVersion is the placeholder recorded when no version has ever
	// been registered yet. Any real release tag compares strictly newer,
	// so the first update bump starts from here.
	FloorVersion = "v0.0.0"
)

// Config mirrors the on-disk config.toml. The TOML key names are literal:
// YUPS_REPO and YUPS_REPO_FALLBACK were chosen by Javi to be reusable
// beyond the update feature.
type Config struct {
	Version          string `toml:"version"`
	YUPSRepo         string `toml:"YUPS_REPO"`
	YUPSRepoFallback string `toml:"YUPS_REPO_FALLBACK"`
}

// Dir returns the yups state directory under the given home directory.
func Dir(home string) string {
	return filepath.Join(home, ".yups")
}

// Path returns the configuration file path under the given home directory.
func Path(home string) string {
	return filepath.Join(Dir(home), "config.toml")
}

// Defaults returns the configuration used when nothing has been stored
// yet. Version starts at the floor: it records the highest version ever
// run, and no version has run until something writes the file.
func Defaults() Config {
	return Config{
		Version:          FloorVersion,
		YUPSRepo:         DefaultYUPSRepo,
		YUPSRepoFallback: DefaultYUPSRepoFallback,
	}
}

// EnsureDefaults fills every empty field with its default value. Fields
// already set by the user are kept untouched.
func EnsureDefaults(c *Config) {
	if c.Version == "" {
		c.Version = FloorVersion
	}
	if c.YUPSRepo == "" {
		c.YUPSRepo = DefaultYUPSRepo
	}
	if c.YUPSRepoFallback == "" {
		c.YUPSRepoFallback = DefaultYUPSRepoFallback
	}
}

// Load reads the configuration file at path. A missing file yields the
// defaults; a file that cannot be parsed is an explicit error (unknown
// keys are tolerated so older files keep working as the format evolves).
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading configuration %q: %w", path, err)
	}

	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("corrupt configuration file %q: %w", path, err)
	}
	EnsureDefaults(&c)
	return c, nil
}

// Save writes the configuration to path, creating the parent directory
// when needed.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", path, err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return fmt.Errorf("encoding configuration for %q: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing configuration %q: %w", path, err)
	}
	return nil
}

// BumpVersion moves c.Version forward to tag when tag is strictly newer,
// reporting whether the stored version changed. It never moves backwards:
// the field records the highest version ever seen so multi-version jumps
// apply every intermediate migration.
func BumpVersion(c *Config, tag string) bool {
	if !semver.IsNewer(c.Version, tag) {
		return false
	}
	c.Version = tag
	return true
}
