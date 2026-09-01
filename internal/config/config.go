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
	"regexp"

	"github.com/BurntSushi/toml"
)

// Default repository URLs written when the configuration file does not
// exist. Forgejo is the canonical source of truth; GitHub is only a
// fallback (approved design decision 3).
const (
	DefaultYUPSRepo                    = "https://code.javilopezg.com/javilopezg/yups"
	DefaultYUPSRepoFallback            = "https://github.com/JaviLopezG/yups"
	DefaultInferenceEndpoint           = "http://localhost:11434"
	DefaultModel                       = "qwen3-coder:latest"
	DefaultAdvancedModel               = "gemma4:latest"
	DefaultLLMEnabled                  = true
	DefaultLLMTimeoutSeconds           = 60
	DefaultToolExecutionTimeoutSeconds = 5
	DefaultMaxToolTurns                = 10
	DefaultMaxToolOutputBytes          = 4096
	DefaultAdvancedMultiplier          = 3
	// FloorVersion is the placeholder recorded when no version has ever
	// been registered yet. Any real release tag compares strictly newer,
	// so the first update bump starts from here.
	FloorVersion = "v0.0.0"
)

// Config mirrors the on-disk config.toml organized into sections.
type Config struct {
	YUPSRepo         string          `toml:"yups-repo"`
	YUPSRepoFallback string          `toml:"yups-repo-fallback"`
	Inference        InferenceConfig `toml:"inference"`
	Limits           LimitsConfig    `toml:"limits"`
}

// InferenceConfig holds AI model and endpoint parameters.
type InferenceConfig struct {
	Endpoint      string `toml:"endpoint"`
	DefaultModel  string `toml:"default-model"`
	AdvancedModel string `toml:"advanced-model"`
	Disabled      bool   `toml:"disabled,omitempty"`
}

// LimitsConfig holds execution limits, timeouts, and multipliers.
type LimitsConfig struct {
	LLMTimeoutSeconds           int `toml:"llm-timeout-seconds,omitempty"`
	ToolExecutionTimeoutSeconds int `toml:"tool-execution-timeout-seconds,omitempty"`
	MaxToolTurns                int `toml:"max-tool-turns,omitempty"`
	MaxToolOutputBytes          int `toml:"max-tool-output-bytes,omitempty"`
	AdvancedMultiplier          int `toml:"advanced-multiplier,omitempty"`
}

// IsLLMEnabled reports whether AI inference is enabled.
func (c Config) IsLLMEnabled() bool {
	return !c.Inference.Disabled
}

// GetInferenceEndpoint returns the configured inference endpoint or default.
func (c Config) GetInferenceEndpoint() string {
	if c.Inference.Endpoint == "" {
		return DefaultInferenceEndpoint
	}
	return c.Inference.Endpoint
}

// GetDefaultModel returns the configured default model name.
func (c Config) GetDefaultModel() string {
	if c.Inference.DefaultModel == "" {
		return DefaultModel
	}
	return c.Inference.DefaultModel
}

// GetAdvancedModel returns the configured advanced model name.
func (c Config) GetAdvancedModel() string {
	if c.Inference.AdvancedModel == "" {
		return DefaultAdvancedModel
	}
	return c.Inference.AdvancedModel
}

// GetLLMTimeoutSeconds returns the configured LLM timeout in seconds.
func (c Config) GetLLMTimeoutSeconds() int {
	if c.Limits.LLMTimeoutSeconds <= 0 {
		return DefaultLLMTimeoutSeconds
	}
	return c.Limits.LLMTimeoutSeconds
}

// GetToolExecutionTimeoutSeconds returns the configured tool timeout in seconds.
func (c Config) GetToolExecutionTimeoutSeconds() int {
	if c.Limits.ToolExecutionTimeoutSeconds <= 0 {
		return DefaultToolExecutionTimeoutSeconds
	}
	return c.Limits.ToolExecutionTimeoutSeconds
}

// GetMaxToolTurns returns the maximum intermediate tool turns allowed.
func (c Config) GetMaxToolTurns() int {
	if c.Limits.MaxToolTurns <= 0 {
		return DefaultMaxToolTurns
	}
	return c.Limits.MaxToolTurns
}

// GetMaxToolOutputBytes returns the maximum output bytes per documentation source.
func (c Config) GetMaxToolOutputBytes() int {
	if c.Limits.MaxToolOutputBytes <= 0 {
		return DefaultMaxToolOutputBytes
	}
	return c.Limits.MaxToolOutputBytes
}

// GetAdvancedMultiplier returns the timeout and turns multiplier for the advanced model.
func (c Config) GetAdvancedMultiplier() int {
	if c.Limits.AdvancedMultiplier <= 0 {
		return DefaultAdvancedMultiplier
	}
	return c.Limits.AdvancedMultiplier
}

// GetAvailableModels returns the configured default and advanced models.
func (c Config) GetAvailableModels() []string {
	models := []string{c.GetDefaultModel(), c.GetAdvancedModel()}
	return dedupeStrings(models)
}

func dedupeStrings(items []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, it := range items {
		if it != "" && !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}

// Dir returns the yups state directory under the given home directory.
func Dir(home string) string {
	return filepath.Join(home, ".yups")
}

// Path returns the configuration file path under the given home directory.
func Path(home string) string {
	return filepath.Join(Dir(home), "config.toml")
}

// CheatsheetsDir returns the directory where downloaded cheatsheets live.
func CheatsheetsDir(home string) string {
	return filepath.Join(Dir(home), "cheatsheets")
}

// LogsDir returns the directory where execution and Ollama interaction logs live.
func LogsDir(home string) string {
	return filepath.Join(Dir(home), "logs")
}

// IncidentsLogPath returns the path to the aggregated incidents log file.
func IncidentsLogPath(home string) string {
	return filepath.Join(LogsDir(home), "incidents.log")
}

// ScriptsDir returns the directory where yups utility scripts live.
func ScriptsDir(home string) string {
	return filepath.Join(Dir(home), "scripts")
}

// ShellDir returns the directory where shell integration scripts live.
func ShellDir(home string) string {
	return filepath.Join(Dir(home), "shell")
}

// ShellScriptPath returns the path to the main bash integration entrypoint script.
func ShellScriptPath(home string) string {
	return filepath.Join(ShellDir(home), "yups.bash")
}

// KeybindingScriptPath returns the path to the readline keybinding integration script.
func KeybindingScriptPath(home string) string {
	return filepath.Join(ShellDir(home), "keybinding.bash")
}

// CompletionScriptPath returns the path to the bash tab-completion script.
func CompletionScriptPath(home string) string {
	return filepath.Join(ShellDir(home), "completion.bash")
}

// EnvScriptPath returns the path to the yups shell wrapper script.
func EnvScriptPath(home string) string {
	return filepath.Join(ShellDir(home), "env.bash")
}

// StatePath returns the path to the internal state.toml file.
func StatePath(home string) string {
	return filepath.Join(Dir(home), "state.toml")
}

// Defaults returns the configuration used when nothing has been stored yet.
func Defaults() Config {
	return Config{
		YUPSRepo:         DefaultYUPSRepo,
		YUPSRepoFallback: DefaultYUPSRepoFallback,
		Inference: InferenceConfig{
			Endpoint:      DefaultInferenceEndpoint,
			DefaultModel:  DefaultModel,
			AdvancedModel: DefaultAdvancedModel,
			Disabled:      false,
		},
		Limits: LimitsConfig{
			LLMTimeoutSeconds:           DefaultLLMTimeoutSeconds,
			ToolExecutionTimeoutSeconds: DefaultToolExecutionTimeoutSeconds,
			MaxToolTurns:                DefaultMaxToolTurns,
			MaxToolOutputBytes:          DefaultMaxToolOutputBytes,
			AdvancedMultiplier:          DefaultAdvancedMultiplier,
		},
	}
}

// EnsureDefaults fills every empty field with its default value. Fields
// already set by the user are kept untouched.
func EnsureDefaults(c *Config) {
	if c.YUPSRepo == "" {
		c.YUPSRepo = DefaultYUPSRepo
	}
	if c.YUPSRepoFallback == "" {
		c.YUPSRepoFallback = DefaultYUPSRepoFallback
	}
	if c.Inference.Endpoint == "" {
		c.Inference.Endpoint = DefaultInferenceEndpoint
	}
	if c.Inference.DefaultModel == "" {
		c.Inference.DefaultModel = DefaultModel
	}
	if c.Inference.AdvancedModel == "" {
		c.Inference.AdvancedModel = DefaultAdvancedModel
	}
	if c.Limits.LLMTimeoutSeconds <= 0 {
		c.Limits.LLMTimeoutSeconds = DefaultLLMTimeoutSeconds
	}
	if c.Limits.ToolExecutionTimeoutSeconds <= 0 {
		c.Limits.ToolExecutionTimeoutSeconds = DefaultToolExecutionTimeoutSeconds
	}
	if c.Limits.MaxToolTurns <= 0 {
		c.Limits.MaxToolTurns = DefaultMaxToolTurns
	}
	if c.Limits.MaxToolOutputBytes <= 0 {
		c.Limits.MaxToolOutputBytes = DefaultMaxToolOutputBytes
	}
	if c.Limits.AdvancedMultiplier <= 0 {
		c.Limits.AdvancedMultiplier = DefaultAdvancedMultiplier
	}
}

// rawLegacyConfig handles parsing both older flat TOML configs and sectioned TOML configs.
type rawLegacyConfig struct {
	YUPSRepo         string `toml:"yups-repo"`
	YUPSRepoFallback string `toml:"yups-repo-fallback"`

	// Flat legacy keys
	InferenceEndpoint           string   `toml:"inference-endpoint"`
	DefaultModel                string   `toml:"default-model"`
	AdvancedModel               string   `toml:"advanced-model"`
	AvailableModels             []string `toml:"available-models"`
	LLMDisabled                 *bool    `toml:"llm-disabled"`
	LLMTimeoutSeconds           int      `toml:"llm-timeout-seconds"`
	ToolExecutionTimeoutSeconds int      `toml:"tool-execution-timeout-seconds"`
	MaxToolTurns                int      `toml:"max-tool-turns"`
	MaxToolOutputBytes          int      `toml:"max-tool-output-bytes"`
	AdvancedMultiplier          int      `toml:"advanced-multiplier"`

	// Sectioned keys
	Inference rawInferenceConfig `toml:"inference"`
	Limits    rawLimitsConfig    `toml:"limits"`
}

type rawInferenceConfig struct {
	Endpoint          string   `toml:"endpoint"`
	InferenceEndpoint string   `toml:"inference-endpoint"`
	DefaultModel      string   `toml:"default-model"`
	AdvancedModel     string   `toml:"advanced-model"`
	AvailableModels   []string `toml:"available-models"`
	Disabled          *bool    `toml:"disabled"`
	LLMDisabled       *bool    `toml:"llm-disabled"`
}

type rawLimitsConfig struct {
	LLMTimeoutSeconds           int `toml:"llm-timeout-seconds"`
	ToolExecutionTimeoutSeconds int `toml:"tool-execution-timeout-seconds"`
	MaxToolTurns                int `toml:"max-tool-turns"`
	MaxToolOutputBytes          int `toml:"max-tool-output-bytes"`
	AdvancedMultiplier          int `toml:"advanced-multiplier"`
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

	var raw rawLegacyConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("corrupt configuration file %q: %w", path, err)
	}

	if versionLineRegex.Match(data) {
		_, _, _ = CleanLegacyVersion(path)
	}
	if availableModelsLineRegex.Match(data) {
		_, _ = CleanLegacyAvailableModels(path)
	}

	c := Config{
		YUPSRepo:         raw.YUPSRepo,
		YUPSRepoFallback: raw.YUPSRepoFallback,
	}

	// Resolve Inference
	endpoint := raw.Inference.Endpoint
	if endpoint == "" {
		endpoint = raw.Inference.InferenceEndpoint
	}
	if endpoint == "" {
		endpoint = raw.InferenceEndpoint
	}
	c.Inference.Endpoint = endpoint

	defaultModel := raw.Inference.DefaultModel
	if defaultModel == "" {
		defaultModel = raw.DefaultModel
	}
	c.Inference.DefaultModel = defaultModel

	advancedModel := raw.Inference.AdvancedModel
	if advancedModel == "" {
		advancedModel = raw.AdvancedModel
	}
	c.Inference.AdvancedModel = advancedModel

	if raw.Inference.Disabled != nil {
		c.Inference.Disabled = *raw.Inference.Disabled
	} else if raw.Inference.LLMDisabled != nil {
		c.Inference.Disabled = *raw.Inference.LLMDisabled
	} else if raw.LLMDisabled != nil {
		c.Inference.Disabled = *raw.LLMDisabled
	}

	// Resolve Limits
	llmTimeout := raw.Limits.LLMTimeoutSeconds
	if llmTimeout <= 0 {
		llmTimeout = raw.LLMTimeoutSeconds
	}
	c.Limits.LLMTimeoutSeconds = llmTimeout

	toolTimeout := raw.Limits.ToolExecutionTimeoutSeconds
	if toolTimeout <= 0 {
		toolTimeout = raw.ToolExecutionTimeoutSeconds
	}
	c.Limits.ToolExecutionTimeoutSeconds = toolTimeout

	maxTurns := raw.Limits.MaxToolTurns
	if maxTurns <= 0 {
		maxTurns = raw.MaxToolTurns
	}
	c.Limits.MaxToolTurns = maxTurns

	maxOutput := raw.Limits.MaxToolOutputBytes
	if maxOutput <= 0 {
		maxOutput = raw.MaxToolOutputBytes
	}
	c.Limits.MaxToolOutputBytes = maxOutput

	advMult := raw.Limits.AdvancedMultiplier
	if advMult <= 0 {
		advMult = raw.AdvancedMultiplier
	}
	c.Limits.AdvancedMultiplier = advMult

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

var versionLineRegex = regexp.MustCompile(`(?m)^version\s*=\s*["']?([^"'\r\n]*)["']?\r?\n?`)

// CleanLegacyVersion removes any legacy `version = "..."` entry from the config
// file at path, preserving comments and formatting. Returns the legacy version found
// (if any) and whether the file was updated.
func CleanLegacyVersion(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	content := string(data)
	match := versionLineRegex.FindStringSubmatch(content)
	if match == nil {
		return "", false, nil
	}
	legacyVer := match[1]
	cleaned := versionLineRegex.ReplaceAllString(content, "")
	if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
		return legacyVer, false, fmt.Errorf("writing cleaned configuration %q: %w", path, err)
	}
	return legacyVer, true, nil
}

var availableModelsLineRegex = regexp.MustCompile(`(?m)^available-models\s*=\s*\[[^\]]*\]\r?\n?`)

// CleanLegacyAvailableModels removes any legacy `available-models = [...]` entry from the config
// file at path, preserving comments and formatting. Returns whether the file was updated.
func CleanLegacyAvailableModels(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	content := string(data)
	if !availableModelsLineRegex.MatchString(content) {
		return false, nil
	}
	cleaned := availableModelsLineRegex.ReplaceAllString(content, "")
	if err := os.WriteFile(path, []byte(cleaned), 0o644); err != nil {
		return false, fmt.Errorf("writing cleaned configuration %q: %w", path, err)
	}
	return true, nil
}
