package app

import (
	"yups/internal/config"
	"yups/internal/semver"
)

// State holds internal operational metadata required for yups execution,
// keeping them separate from user preferences in config.toml.
type State struct {
	Version         string            `toml:"version"`
	LastApplied     string            `toml:"last-applied"`
	AvailableModels []string          `toml:"available-models,omitempty"`
	Cheatsheets     map[string]string `toml:"cheatsheets,omitempty"`
}

// GetAvailableModels returns the list of available models stored in state or fallbacks.
func (s *State) GetAvailableModels() []string {
	if s != nil && len(s.AvailableModels) > 0 {
		return s.AvailableModels
	}
	return []string{config.DefaultModel, config.DefaultAdvancedModel}
}

// BumpVersion moves s.Version forward to tag when tag is strictly newer,
// reporting whether the stored version changed.
func (s *State) BumpVersion(tag string) bool {
	if s.Version == "" {
		s.Version = config.FloorVersion
	}
	if !semver.IsNewer(s.Version, tag) {
		return false
	}
	s.Version = tag
	return true
}

// SetVersion updates s.Version to tag, returning whether the stored version changed.
func (s *State) SetVersion(tag string) bool {
	if s.Version == tag {
		return false
	}
	s.Version = tag
	return true
}
