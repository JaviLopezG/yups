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
	case c.Version != FloorVersion:
		t.Errorf("Version = %q, want floor %q", c.Version, FloorVersion)
	case c.YUPSRepo != DefaultYUPSRepo:
		t.Errorf("YUPSRepo = %q, want %q", c.YUPSRepo, DefaultYUPSRepo)
	case c.YUPSRepoFallback != DefaultYUPSRepoFallback:
		t.Errorf("YUPSRepoFallback = %q, want %q", c.YUPSRepoFallback, DefaultYUPSRepoFallback)
	case c.InferenceEndpoint != DefaultInferenceEndpoint:
		t.Errorf("InferenceEndpoint = %q, want %q", c.InferenceEndpoint, DefaultInferenceEndpoint)
	case c.DefaultModel != DefaultModel:
		t.Errorf("DefaultModel = %q, want %q", c.DefaultModel, DefaultModel)
	case c.AdvancedModel != DefaultAdvancedModel:
		t.Errorf("AdvancedModel = %q, want %q", c.AdvancedModel, DefaultAdvancedModel)
	}
}

func TestPathsLiveUnderDotYups(t *testing.T) {
	if got, want := Dir("/home/u"), "/home/u/.yups"; got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
	if got, want := Path("/home/u"), "/home/u/.yups/config.toml"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
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

func TestLoadParsesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "version = \"v1.2.3\"\n" +
		"yups-repo = \"https://example.com/a/b\"\n" +
		"yups-repo-fallback = \"https://example.org/c/d\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	switch {
	case c.Version != "v1.2.3":
		t.Errorf("Version = %q, want v1.2.3", c.Version)
	case c.YUPSRepo != "https://example.com/a/b":
		t.Errorf("YUPSRepo = %q, want https://example.com/a/b", c.YUPSRepo)
	case c.YUPSRepoFallback != "https://example.org/c/d":
		t.Errorf("YUPSRepoFallback = %q, want https://example.org/c/d", c.YUPSRepoFallback)
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
		Version:                     "v9.9.9",
		YUPSRepo:                    "https://r.example/x",
		YUPSRepoFallback:            "https://f.example/y",
		InferenceEndpoint:           "http://custom:11434",
		DefaultModel:                "custom-coder:latest",
		AdvancedModel:               "custom-adv:latest",
		LLMDisabled:                 false,
		LLMTimeoutSeconds:           DefaultLLMTimeoutSeconds,
		ToolExecutionTimeoutSeconds: DefaultToolExecutionTimeoutSeconds,
		MaxToolTurns:                DefaultMaxToolTurns,
		MaxToolOutputBytes:          DefaultMaxToolOutputBytes,
		AdvancedMultiplier:          DefaultAdvancedMultiplier,
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
	content := "llm-disabled = true\n"
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
	c := Config{Version: "", YUPSRepo: "https://keep.example/a", YUPSRepoFallback: ""}
	EnsureDefaults(&c)
	switch {
	case c.Version != FloorVersion:
		t.Errorf("empty Version not filled: %q", c.Version)
	case c.YUPSRepo != "https://keep.example/a":
		t.Errorf("user YUPSRepo overwritten: %q", c.YUPSRepo)
	case c.YUPSRepoFallback != DefaultYUPSRepoFallback:
		t.Errorf("empty YUPSRepoFallback not filled: %q", c.YUPSRepoFallback)
	}
}

func TestBumpVersionNeverMovesBackwards(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		tag       string
		wantBump  bool
		wantFinal string
	}{
		{"newer tag bumps", "v1.0.0", "v1.2.0", true, "v1.2.0"},
		{"equal tag keeps", "v1.2.0", "v1.2.0", false, "v1.2.0"},
		{"older tag keeps", "v1.2.0", "v1.0.0", false, "v1.2.0"},
		{"dev current keeps older tag", "dev", "v0.1.0", false, "dev"},
		{"floor accepts first real tag", FloorVersion, "v1.0.0", true, "v1.0.0"},
		{"multi version jump lands on newest", "v1.0.0", "v3.0.0", true, "v3.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Version: tt.current}
			got := BumpVersion(&c, tt.tag)
			if got != tt.wantBump {
				t.Errorf("BumpVersion(%q -> %q) = %v, want %v", tt.current, tt.tag, got, tt.wantBump)
			}
			if c.Version != tt.wantFinal {
				t.Errorf("final Version = %q, want %q", c.Version, tt.wantFinal)
			}
		})
	}
}

func TestSetVersion(t *testing.T) {
	c := Config{Version: "v1.0.0"}
	if !SetVersion(&c, "v0.5.0") {
		t.Error("SetVersion to older version should return true")
	}
	if c.Version != "v0.5.0" {
		t.Errorf("Version = %q, want v0.5.0", c.Version)
	}
	if SetVersion(&c, "v0.5.0") {
		t.Error("SetVersion to same version should return false")
	}
}

func TestAvailableModelsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "available-models = [\"qwen2.5-coder:7b\", \"qwen3.8:latest\", \"gemma3:latest\"]\n"
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
