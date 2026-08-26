// Package semver provides lightweight semantic version parsing and comparison
// for vX.Y.Z tags without external dependencies.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a parsed semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	IsDev      bool
	Raw        string
}

// Parse parses a semantic version string like "v1.2.3", "1.2.3", "v1.0.0-rc.1", or "dev".
func Parse(v string) (Version, error) {
	raw := strings.TrimSpace(v)
	if raw == "dev" {
		return Version{IsDev: true, Raw: raw}, nil
	}
	if raw == "" {
		return Version{}, fmt.Errorf("empty version string")
	}

	clean := strings.TrimPrefix(raw, "v")
	clean = strings.TrimPrefix(clean, "V")

	// Split off pre-release if present
	preRelease := ""
	if idx := strings.Index(clean, "-"); idx != -1 {
		preRelease = clean[idx+1:]
		clean = clean[:idx]
	}

	parts := strings.Split(clean, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid semver %q: expected 3 parts (Major.Minor.Patch)", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("invalid major version %q in %q", parts[0], raw)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("invalid minor version %q in %q", parts[1], raw)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, fmt.Errorf("invalid patch version %q in %q", parts[2], raw)
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
		IsDev:      false,
		Raw:        raw,
	}, nil
}

// Compare compares two version strings.
// It returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
//
// "dev" is treated as higher than any numbered semantic version to facilitate
// testing. Invalid versions are treated as lower than any valid semantic version.
// If both are invalid or both are "dev", Compare returns 0 when equal, or
// compares them lexicographically.
func Compare(v1, v2 string) int {
	ver1, err1 := Parse(v1)
	ver2, err2 := Parse(v2)

	if err1 != nil && err2 != nil {
		return strings.Compare(v1, v2)
	}
	if err1 != nil {
		return -1
	}
	if err2 != nil {
		return 1
	}

	if ver1.IsDev && ver2.IsDev {
		return strings.Compare(v1, v2)
	}
	if ver1.IsDev {
		return 1
	}
	if ver2.IsDev {
		return -1
	}

	if ver1.Major != ver2.Major {
		if ver1.Major < ver2.Major {
			return -1
		}
		return 1
	}
	if ver1.Minor != ver2.Minor {
		if ver1.Minor < ver2.Minor {
			return -1
		}
		return 1
	}
	if ver1.Patch != ver2.Patch {
		if ver1.Patch < ver2.Patch {
			return -1
		}
		return 1
	}

	// Normal versions with no pre-release have higher precedence than pre-release
	if ver1.PreRelease == "" && ver2.PreRelease != "" {
		return 1
	}
	if ver1.PreRelease != "" && ver2.PreRelease == "" {
		return -1
	}
	if ver1.PreRelease != ver2.PreRelease {
		return strings.Compare(ver1.PreRelease, ver2.PreRelease)
	}

	return 0
}

// IsNewer reports whether candidate is strictly newer than current.
func IsNewer(current, candidate string) bool {
	return Compare(candidate, current) > 0
}
