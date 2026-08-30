package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultsFillEveryField(t *testing.T) {
	c := Defaults()
	switch {
	case c.YUPSRepo != DefaultYUPSRepo:
		t.Errorf("YUPSRepo = %q, want %q", c.YUPSRepo, DefaultYUPSRepo)
	case c.YUPSRepoFallback != DefaultYUPSRepoFallback:
		t.Errorf("YUPSRepoFallback = %q, want %q", c.YUPSRepoFallback, DefaultYUPSRepoFallback)
	case c.Inference.Endpoint != DefaultInferenceEndpoint:
		t.Errorf("Inference.Endpoint = %q, want %q", c.Inference.Endpoint, DefaultInferenceEndpoint)
	case c.Inference.DefaultModel != DefaultModel:
		t.Errorf("Inference.DefaultModel = %q, want %q", c.Inference.DefaultModel, DefaultModel)
	case c.Inference.AdvancedModel != DefaultAdvancedModel:
		t.Errorf("Inference.AdvancedModel = %q, want %q", c.Inference.AdvancedModel, DefaultAdvancedModel)
	case c.Limits.LLMTimeoutSeconds != DefaultLLMTimeoutSeconds:
		t.Errorf("Limits.LLMTimeoutSeconds = %d, want %d", c.Limits.LLMTimeoutSeconds, DefaultLLMTimeoutSeconds)
	case c.Limits.ToolExecutionTimeoutSeconds != DefaultToolExecutionTimeoutSeconds:
		t.Errorf("Limits.ToolExecutionTimeoutSeconds = %d, want %d", c.Limits.ToolExecutionTimeoutSeconds, DefaultToolExecutionTimeoutSeconds)
	case c.Limits.MaxToolTurns != DefaultMaxToolTurns:
		t.Errorf("Limits.MaxToolTurns = %d, want %d", c.Limits.MaxToolTurns, DefaultMaxToolTurns)
	case c.Limits.MaxToolOutputBytes != DefaultMaxToolOutputBytes:
		t.Errorf("Limits.MaxToolOutputBytes = %d, want %d", c.Limits.MaxToolOutputBytes, DefaultMaxToolOutputBytes)
	case c.Limits.AdvancedMultiplier != DefaultAdvancedMultiplier:
		t.Errorf("Limits.AdvancedMultiplier = %d, want %d", c.Limits.AdvancedMultiplier, DefaultAdvancedMultiplier)
	}
}

func TestPathsLiveUnderDotYups(t *testing.T) {
	if got, want := Dir("/home/u"), "/home/u/.yups"; got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
	if got, want := Path("/home/u"), "/home/u/.yups/config.toml"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	if got, want := StatePath("/home/u"), "/home/u/.yups/state.toml"; got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
	if got, want := LogsDir("/home/u"), "/home/u/.yups/logs"; got != want {
		t.Errorf("LogsDir = %q, want %q", got, want)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if !reflect.DeepEqual(c, Defaults()) {
		t.Errorf("Load missing file = %+v, want defaults %+v", c, Defaults())
	}
}

func TestLoadParsesSectionedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "yups-repo = \"https://example.com/a/b\"\n" +
		"yups-repo-fallback = \"https://example.org/c/d\"\n\n" +
		"[inference]\n" +
		"endpoint = \"http://remote:11434\"\n" +
		"default-model = \"qwen2.5-coder:7b\"\n" +
		"advanced-model = \"qwen3.8:latest\"\n\n" +
		"[limits]\n" +
		"llm-timeout-seconds = 45\n" +
		"tool-execution-timeout-seconds = 20\n" +
		"max-tool-turns = 8\n" +
		"max-tool-output-bytes = 2048\n" +
		"advanced-multiplier = 2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	switch {
	case c.YUPSRepo != "https://example.com/a/b":
		t.Errorf("YUPSRepo = %q, want https://example.com/a/b", c.YUPSRepo)
	case c.YUPSRepoFallback != "https://example.org/c/d":
		t.Errorf("YUPSRepoFallback = %q, want https://example.org/c/d", c.YUPSRepoFallback)
	case c.GetInferenceEndpoint() != "http://remote:11434":
		t.Errorf("InferenceEndpoint = %q, want http://remote:11434", c.GetInferenceEndpoint())
	case c.GetDefaultModel() != "qwen2.5-coder:7b":
		t.Errorf("DefaultModel = %q, want qwen2.5-coder:7b", c.GetDefaultModel())
	case c.GetAdvancedModel() != "qwen3.8:latest":
		t.Errorf("AdvancedModel = %q, want qwen3.8:latest", c.GetAdvancedModel())
	case c.GetLLMTimeoutSeconds() != 45:
		t.Errorf("LLMTimeoutSeconds = %d, want 45", c.GetLLMTimeoutSeconds())
	case c.GetToolExecutionTimeoutSeconds() != 20:
		t.Errorf("ToolExecutionTimeoutSeconds = %d, want 20", c.GetToolExecutionTimeoutSeconds())
	case c.GetMaxToolTurns() != 8:
		t.Errorf("MaxToolTurns = %d, want 8", c.GetMaxToolTurns())
	case c.GetMaxToolOutputBytes() != 2048:
		t.Errorf("MaxToolOutputBytes = %d, want 2048", c.GetMaxToolOutputBytes())
	case c.GetAdvancedMultiplier() != 2:
		t.Errorf("AdvancedMultiplier = %d, want 2", c.GetAdvancedMultiplier())
	}
}

func TestLoadParsesLegacyFlatFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "yups-repo = \"https://example.com/a/b\"\n" +
		"yups-repo-fallback = \"https://example.org/c/d\"\n" +
		"inference-endpoint = \"http://legacy:11434\"\n" +
		"default-model = \"legacy-coder:latest\"\n" +
		"advanced-model = \"legacy-adv:latest\"\n" +
		"llm-timeout-seconds = 90\n" +
		"tool-execution-timeout-seconds = 15\n" +
		"max-tool-turns = 5\n" +
		"max-tool-output-bytes = 1024\n" +
		"advanced-multiplier = 4\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	switch {
	case c.YUPSRepo != "https://example.com/a/b":
		t.Errorf("YUPSRepo = %q, want https://example.com/a/b", c.YUPSRepo)
	case c.YUPSRepoFallback != "https://example.org/c/d":
		t.Errorf("YUPSRepoFallback = %q, want https://example.org/c/d", c.YUPSRepoFallback)
	case c.Inference.Endpoint != "http://legacy:11434":
		t.Errorf("Inference.Endpoint = %q, want http://legacy:11434", c.Inference.Endpoint)
	case c.Inference.DefaultModel != "legacy-coder:latest":
		t.Errorf("Inference.DefaultModel = %q, want legacy-coder:latest", c.Inference.DefaultModel)
	case c.Inference.AdvancedModel != "legacy-adv:latest":
		t.Errorf("Inference.AdvancedModel = %q, want legacy-adv:latest", c.Inference.AdvancedModel)
	case c.Limits.LLMTimeoutSeconds != 90:
		t.Errorf("Limits.LLMTimeoutSeconds = %d, want 90", c.Limits.LLMTimeoutSeconds)
	case c.Limits.ToolExecutionTimeoutSeconds != 15:
		t.Errorf("Limits.ToolExecutionTimeoutSeconds = %d, want 15", c.Limits.ToolExecutionTimeoutSeconds)
	case c.Limits.MaxToolTurns != 5:
		t.Errorf("Limits.MaxToolTurns = %d, want 5", c.Limits.MaxToolTurns)
	case c.Limits.MaxToolOutputBytes != 1024:
		t.Errorf("Limits.MaxToolOutputBytes = %d, want 1024", c.Limits.MaxToolOutputBytes)
	case c.Limits.AdvancedMultiplier != 4:
		t.Errorf("Limits.AdvancedMultiplier = %d, want 4", c.Limits.AdvancedMultiplier)
	}
}

func TestLoadCorruptFileReturnsExplicitError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("this is [ not toml"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	c, err := Load(path)
	if err == nil {
		t.Fatal("Load corrupt file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("error %q does not mention the file is corrupt", err)
	}
	if !reflect.DeepEqual(c, Config{}) {
		t.Errorf("corrupt load returned non-zero config %+v", c)
	}
}

func TestSaveThenLoadRoundtripCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".yups", "config.toml")
	want := Config{
		YUPSRepo:         "https://r.example/x",
		YUPSRepoFallback: "https://f.example/y",
		Inference: InferenceConfig{
			Endpoint:      "http://custom:11434",
			DefaultModel:  "custom-coder:latest",
			AdvancedModel: "custom-adv:latest",
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

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roundtrip = %+v, want %+v", got, want)
	}
}

func TestLLMDisabledConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "[inference]\ndisabled = true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.IsLLMEnabled() {
		t.Error("expected LLM to be disabled, got enabled")
	}
}

func TestEnsureDefaultsKeepsUserValues(t *testing.T) {
	c := Config{YUPSRepo: "https://keep.example/a", YUPSRepoFallback: ""}
	EnsureDefaults(&c)
	switch {
	case c.YUPSRepo != "https://keep.example/a":
		t.Errorf("user YUPSRepo overwritten: %q", c.YUPSRepo)
	case c.YUPSRepoFallback != DefaultYUPSRepoFallback:
		t.Errorf("empty YUPSRepoFallback not filled: %q", c.YUPSRepoFallback)
	}
}

func TestAvailableModelsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "[inference]\navailable-models = [\"qwen2.5-coder:7b\", \"qwen3.8:latest\", \"gemma3:latest\"]\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	models := c.GetAvailableModels()
	if len(models) != 3 || models[0] != "qwen2.5-coder:7b" || models[1] != "qwen3.8:latest" || models[2] != "gemma3:latest" {
		t.Errorf("GetAvailableModels() = %v, want 3 models", models)
	}
}

func TestCleanLegacyVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "# Top comment\nversion = \"v0.6.5\"\n\nyups-repo = \"https://example.com/repo\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	ver, cleaned, err := CleanLegacyVersion(path)
	if err != nil {
		t.Fatalf("CleanLegacyVersion failed: %v", err)
	}
	if !cleaned || ver != "v0.6.5" {
		t.Errorf("cleaned = %v, ver = %q, want true, v0.6.5", cleaned, ver)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading cleaned file: %v", err)
	}
	if strings.Contains(string(data), "version =") {
		t.Errorf("version still present in file:\n%s", string(data))
	}
	if !strings.Contains(string(data), "# Top comment") {
		t.Errorf("comment was lost:\n%s", string(data))
	}
}

func TestLoadCleansLegacyVersionAutomatically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "version = \"v0.9.0\"\nyups-repo = \"https://example.com/a/b\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.YUPSRepo != "https://example.com/a/b" {
		t.Errorf("YUPSRepo = %q, want https://example.com/a/b", c.YUPSRepo)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file after load: %v", err)
	}
	if strings.Contains(string(data), "version =") {
		t.Errorf("Load did not clean version from file:\n%s", string(data))
	}
}
