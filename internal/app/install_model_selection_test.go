package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"yups/internal/config"
	"yups/internal/llm"
)

func TestFindInstalledModel(t *testing.T) {
	models := []llm.ModelInfo{
		{Name: "gemma4:26b"},
		{Name: "gemma4:latest"},
		{Name: "qwen3-coder"},
	}

	if got := findInstalledModel(models, "gemma4:latest"); got != "gemma4:latest" {
		t.Errorf("findInstalledModel for gemma4:latest = %q, want gemma4:latest", got)
	}
	if got := findInstalledModel(models, "qwen3-coder:latest"); got != "qwen3-coder" {
		t.Errorf("findInstalledModel for qwen3-coder:latest = %q, want qwen3-coder", got)
	}
	if got := findInstalledModel(models, "nonexistent:latest"); got != "" {
		t.Errorf("findInstalledModel for nonexistent = %q, want empty", got)
	}
}

func TestFindFirstFamilyModel(t *testing.T) {
	models := []llm.ModelInfo{
		{Name: "other-model:latest"},
		{Name: "gemma4:26b"},
		{Name: "gemma4:latest"},
		{Name: "qooba/qwen3-coder-30b-a3b-instruct:q3_k_m"},
	}

	if got := findFirstFamilyModel(models, "gemma"); got != "gemma4:26b" {
		t.Errorf("findFirstFamilyModel(gemma) = %q, want gemma4:26b", got)
	}
	if got := findFirstFamilyModel(models, "qwen"); got != "qooba/qwen3-coder-30b-a3b-instruct:q3_k_m" {
		t.Errorf("findFirstFamilyModel(qwen) = %q, want qooba/qwen3-coder-30b-a3b-instruct:q3_k_m", got)
	}
	if got := findFirstFamilyModel(models, "llama"); got != "" {
		t.Errorf("findFirstFamilyModel(llama) = %q, want empty", got)
	}
}

func TestResolveModelSlotExactInstalled(t *testing.T) {
	models := []llm.ModelInfo{
		{Name: "gemma4:26b"},
		{Name: "gemma4:latest"},
	}

	var stdout bytes.Buffer
	// When target model (gemma4:latest) is installed, it should use it without asking questions
	env := &Env{
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			t.Fatalf("unexpected AskConfirmation call: %s", prompt)
			return false
		},
	}

	got := resolveModelSlot(env, nil, "advanced", config.DefaultAdvancedModel, "gemma", models, nil, &stdout)
	if got != "gemma4:latest" {
		t.Errorf("resolveModelSlot = %q, want gemma4:latest", got)
	}
}

func TestResolveModelSlotSuggestPullAccepted(t *testing.T) {
	models := []llm.ModelInfo{
		{Name: "gemma4:26b"},
	}

	pullCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/pull" {
			pullCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "success"}`))
		}
	}))
	defer ts.Close()

	llmClient := llm.NewClient(ts.Client(), ts.URL)
	var stdout bytes.Buffer
	env := &Env{
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			return true // Accept pull
		},
	}

	var discovered []string
	got := resolveModelSlot(env, llmClient, "advanced", config.DefaultAdvancedModel, "gemma", models, &discovered, &stdout)
	if got != config.DefaultAdvancedModel {
		t.Errorf("resolveModelSlot = %q, want %q", got, config.DefaultAdvancedModel)
	}
	if !pullCalled {
		t.Errorf("expected PullModel to be called")
	}
}

func TestResolveModelSlotDeclinePullSuggestsFamily(t *testing.T) {
	models := []llm.ModelInfo{
		{Name: "qooba/qwen3-coder-30b-a3b-instruct:q3_k_m"},
		{Name: "other-model:latest"},
	}

	var stdout bytes.Buffer
	promptAsked := false
	suggestedDefault := ""
	env := &Env{
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			return false // Decline pull
		},
		AskPrompt: func(prompt, defaultValue string) string {
			promptAsked = true
			suggestedDefault = defaultValue
			return defaultValue // Accept suggested default
		},
	}

	got := resolveModelSlot(env, nil, "default", config.DefaultModel, "qwen", models, nil, &stdout)
	if got != "qooba/qwen3-coder-30b-a3b-instruct:q3_k_m" {
		t.Errorf("resolveModelSlot = %q, want qooba/qwen3-coder-30b-a3b-instruct:q3_k_m", got)
	}
	if !promptAsked {
		t.Errorf("expected AskPrompt to be called")
	}
	if suggestedDefault != "qooba/qwen3-coder-30b-a3b-instruct:q3_k_m" {
		t.Errorf("suggestedDefault = %q, want qooba/qwen3-coder-30b-a3b-instruct:q3_k_m", suggestedDefault)
	}
}
