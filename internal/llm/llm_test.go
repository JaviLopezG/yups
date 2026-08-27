package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListModels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %q, want /api/tags", r.URL.Path)
		}
		data := tagsResponse{
			Models: []struct {
				Name    string `json:"name"`
				Model   string `json:"model"`
				Size    int64  `json:"size"`
				Digest  string `json:"digest"`
				Details struct {
					Family        string `json:"family"`
					ParameterSize string `json:"parameter_size"`
				} `json:"details"`
			}{
				{
					Name: "qwen2.5-coder:7b",
					Size: 4500000000,
					Details: struct {
						Family        string `json:"family"`
						ParameterSize string `json:"parameter_size"`
					}{Family: "qwen2", ParameterSize: "7B"},
				},
				{
					Name: "gemma4:latest",
					Size: 8000000000,
					Details: struct {
						Family        string `json:"family"`
						ParameterSize string `json:"parameter_size"`
					}{Family: "gemma", ParameterSize: "9B"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer ts.Close()

	client := NewClient(ts.Client(), ts.URL)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models count = %d, want 2", len(models))
	}
	if models[0].Name != "qwen2.5-coder:7b" {
		t.Errorf("model 0 name = %q, want qwen2.5-coder:7b", models[0].Name)
	}
}

func TestClientChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q, want /api/chat", r.URL.Path)
		}
		resp := ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: Message{
				Role:    "assistant",
				Content: "The -Z option is used for SELinux context.\nSuggested command: ls -Z",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.Client(), ts.URL)
	chatResp, err := client.Chat(context.Background(), ChatRequest{
		Model: "qwen2.5-coder:7b",
		Messages: []Message{
			{Role: "user", Content: "What is -Z in ls?"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if !strings.Contains(chatResp.Message.Content, "SELinux") {
		t.Errorf("chat response = %q, want containing 'SELinux'", chatResp.Message.Content)
	}
}

func TestSelectBestModels(t *testing.T) {
	tests := []struct {
		name         string
		models       []ModelInfo
		wantDefault  string
		wantAdvanced string
	}{
		{
			name:         "empty list uses fallbacks",
			models:       nil,
			wantDefault:  FallbackDefaultModel,
			wantAdvanced: FallbackAdvancedModel,
		},
		{
			name: "single model used for both",
			models: []ModelInfo{
				{Name: "llama3.2:latest", Size: 2000000000},
			},
			wantDefault:  "llama3.2:latest",
			wantAdvanced: "llama3.2:latest",
		},
		{
			name: "coder model preferred for default",
			models: []ModelInfo{
				{Name: "llama3:latest", Size: 4000000000},
				{Name: "qwen2.5-coder:7b", Size: 4500000000},
				{Name: "gemma4:13b", Size: 9000000000},
			},
			wantDefault:  "qwen2.5-coder:7b",
			wantAdvanced: "gemma4:13b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def, adv := SelectBestModels(tc.models)
			if def != tc.wantDefault {
				t.Errorf("default model = %q, want %q", def, tc.wantDefault)
			}
			if adv != tc.wantAdvanced {
				t.Errorf("advanced model = %q, want %q", adv, tc.wantAdvanced)
			}
		})
	}
}

type fakeLLMEnv struct {
	home      string
	cwd       string
	osRelease string
	history   []string
	snippets  map[string]string
	dirItems  map[string][]string
}

func (f *fakeLLMEnv) UserHomeDir() (string, error)                   { return f.home, nil }
func (f *fakeLLMEnv) Getwd() (string, error)                         { return f.cwd, nil }
func (f *fakeLLMEnv) ReadOSRelease() string                          { return f.osRelease }
func (f *fakeLLMEnv) ReadHistory(home string, maxLines int) []string { return f.history }
func (f *fakeLLMEnv) ReadFileSnippet(path string, max int) (string, error) {
	return f.snippets[path], nil
}
func (f *fakeLLMEnv) ListDirNames(dir string, max int) []string {
	return f.dirItems[dir]
}

func TestGatherContext(t *testing.T) {
	env := &fakeLLMEnv{
		home:      "/home/alice",
		cwd:       "/home/alice/project",
		osRelease: "Ubuntu 24.04 LTS",
		history:   []string{"git status", "make test"},
		dirItems: map[string][]string{
			".":  {"main.go", "go.mod", "script.sh"},
			"..": {"project", "other"},
		},
		snippets: map[string]string{
			"script.sh": "#!/bin/bash\necho hello",
		},
	}

	ctx := GatherContext(env, []string{"script.sh"})
	if ctx.OSRelease != "Ubuntu 24.04 LTS" {
		t.Errorf("OSRelease = %q", ctx.OSRelease)
	}
	if ctx.CWD != "/home/alice/project" {
		t.Errorf("CWD = %q", ctx.CWD)
	}
	if len(ctx.CWDListing) != 3 {
		t.Errorf("CWDListing = %v", ctx.CWDListing)
	}
	if len(ctx.RecentHistory) != 2 {
		t.Errorf("RecentHistory = %v", ctx.RecentHistory)
	}
	if snippet, ok := ctx.FileSnippets["script.sh"]; !ok || !strings.Contains(snippet, "#!/bin/bash") {
		t.Errorf("FileSnippets[script.sh] = %q", snippet)
	}
}

func TestParseLLMResponse(t *testing.T) {
	raw := `The option -Z enables SELinux security context display for files.
It was likely what you intended instead of -9.

Suggested command: ls -laZ /var/log`

	res := ParseLLMResponse(raw)
	if res.SuggestedCommand != "ls -laZ /var/log" {
		t.Errorf("SuggestedCommand = %q, want 'ls -laZ /var/log'", res.SuggestedCommand)
	}
	if !strings.Contains(res.Explanation, "SELinux") {
		t.Errorf("Explanation = %q", res.Explanation)
	}
}

func TestParseLLMResponseWithScript(t *testing.T) {
	raw := "Here is a script to achieve what you want:\n```bash\nfor f in *.txt; do\n  echo \"$f\"\ndone\n```"
	res := ParseLLMResponse(raw)
	if !strings.Contains(res.SuggestedScript, "for f in *.txt") {
		t.Errorf("SuggestedScript = %q", res.SuggestedScript)
	}
}
