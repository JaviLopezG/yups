package llm

import (
	"bytes"
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
					}{Family: "gemma", ParameterSize: "13B"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer ts.Close()

	c := NewClient(ts.Client(), ts.URL)
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].Name != "qwen2.5-coder:7b" {
		t.Errorf("models[0] = %q, want qwen2.5-coder:7b", models[0].Name)
	}
}

func TestClientListRunningModelsAndIsModelLoaded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ps" {
			t.Errorf("path = %q, want /api/ps", r.URL.Path)
		}
		data := psResponse{
			Models: []struct {
				Name     string `json:"name"`
				Model    string `json:"model"`
				Size     int64  `json:"size"`
				SizeVRAM int64  `json:"size_vram"`
				Digest   string `json:"digest"`
			}{
				{
					Name:     "qwen3.8:latest",
					Model:    "qwen3.8:latest",
					Size:     5000000000,
					SizeVRAM: 5000000000,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer ts.Close()

	c := NewClient(ts.Client(), ts.URL)
	running, err := c.ListRunningModels(context.Background())
	if err != nil {
		t.Fatalf("ListRunningModels: %v", err)
	}
	if len(running) != 1 {
		t.Fatalf("got %d running models, want 1", len(running))
	}
	if running[0].Name != "qwen3.8:latest" {
		t.Errorf("running[0] = %q, want qwen3.8:latest", running[0].Name)
	}

	if !c.IsModelLoaded(context.Background(), "qwen3.8:latest") {
		t.Error("expected qwen3.8:latest to be loaded")
	}
	if !c.IsModelLoaded(context.Background(), "qwen3.8") {
		t.Error("expected qwen3.8 without tag to be reported loaded")
	}
	if c.IsModelLoaded(context.Background(), "nonexistent:latest") {
		t.Error("expected nonexistent model to NOT be loaded")
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
		{
			name: "qwen and gemma prioritized over codestral and others",
			models: []ModelInfo{
				{Name: "phi4:14b", Size: 8000000000},
				{Name: "codestral:latest", Size: 14000000000},
				{Name: "qwen3-coder:latest", Size: 7000000000},
				{Name: "gemma4:latest", Size: 10000000000},
			},
			wantDefault:  "qwen3-coder:latest",
			wantAdvanced: "gemma4:latest",
		},
		{
			name: "qwen3.8 prioritized for advanced",
			models: []ModelInfo{
				{Name: "qwen2.5-coder:7b", Size: 4500000000},
				{Name: "qwen3.8:latest", Size: 12000000000},
				{Name: "gemma4:latest", Size: 10000000000},
			},
			wantDefault:  "qwen2.5-coder:7b",
			wantAdvanced: "qwen3.8:latest",
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

func TestClientPullModel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["name"] != "qwen2.5-coder:7b" {
			t.Errorf("pull model name = %v, want qwen2.5-coder:7b", payload["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(PullProgress{
			Status:    "pulling manifest",
			Total:     100,
			Completed: 50,
		})
		_ = json.NewEncoder(w).Encode(PullProgress{
			Status:    "success",
			Total:     100,
			Completed: 100,
		})
	}))
	defer ts.Close()

	client := NewClient(ts.Client(), ts.URL)
	var progress bytes.Buffer
	err := client.PullModel(context.Background(), "qwen2.5-coder:7b", &progress)
	if err != nil {
		t.Fatalf("PullModel failed: %v", err)
	}

	out := progress.String()
	if !strings.Contains(out, "pulling manifest") || !strings.Contains(out, "success") {
		t.Errorf("unexpected progress output:\n%s", out)
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

func TestParseLLMResponseJSON(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantCmd     string
		wantScript  string
		wantExplain string
	}{
		{
			name: "strict json with kebab-case",
			raw: `{
				"explanation": "The -Z option is used for SELinux context.",
				"suggested-command": "ls -laZ /var/log",
				"suggested-script": ""
			}`,
			wantCmd:     "ls -laZ /var/log",
			wantExplain: "The -Z option is used for SELinux context.",
		},
		{
			name: "json with snake_case",
			raw: `{
				"explanation": "Typo in command.",
				"suggested_command": "ls -av"
			}`,
			wantCmd:     "ls -av",
			wantExplain: "Typo in command.",
		},
		{
			name: "json with camelCase",
			raw: `{
				"explanation": "Use find instead.",
				"suggestedCommand": "find . -name '*.txt'"
			}`,
			wantCmd:     "find . -name '*.txt'",
			wantExplain: "Use find instead.",
		},
		{
			name: "json with multiline script",
			raw: `{
				"explanation": "Iterate through text files.",
				"suggested-script": "for f in *.txt; do\n  echo \"$f\"\ndone"
			}`,
			wantScript:  "for f in *.txt; do\n  echo \"$f\"\ndone",
			wantExplain: "Iterate through text files.",
		},
		{
			name: "json with single-line script normalized to command",
			raw: `{
				"explanation": "Single line script.",
				"suggested-script": "tar -czf archive.tar.gz /path/to/dir"
			}`,
			wantCmd:     "tar -czf archive.tar.gz /path/to/dir",
			wantExplain: "Single line script.",
		},
		{
			name:        "markdown fenced json",
			raw:         "```json\n{\n  \"explanation\": \"Fenced JSON response.\",\n  \"suggested-command\": \"grep -rn 'pattern' .\"\n}\n```",
			wantCmd:     "grep -rn 'pattern' .",
			wantExplain: "Fenced JSON response.",
		},
		{
			name:        "markdown fenced json without language tag",
			raw:         "```\n{\n  \"explanation\": \"Fenced without tag.\",\n  \"suggested-command\": \"ps aux | grep node\"\n}\n```",
			wantCmd:     "ps aux | grep node",
			wantExplain: "Fenced without tag.",
		},
		{
			name:        "json with surrounding prose text",
			raw:         "Here is the result:\n{\n  \"explanation\": \"Answer with prose wrapper.\",\n  \"suggested-command\": \"uptime\"\n}\nHope this helps!",
			wantCmd:     "uptime",
			wantExplain: "Answer with prose wrapper.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := ParseLLMResponse(tc.raw)
			if res.SuggestedCommand != tc.wantCmd {
				t.Errorf("SuggestedCommand = %q, want %q", res.SuggestedCommand, tc.wantCmd)
			}
			if res.SuggestedScript != tc.wantScript {
				t.Errorf("SuggestedScript = %q, want %q", res.SuggestedScript, tc.wantScript)
			}
			if res.Explanation != tc.wantExplain {
				t.Errorf("Explanation = %q, want %q", res.Explanation, tc.wantExplain)
			}
		})
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

func TestBuildChatRequestIncludesTools(t *testing.T) {
	sysCtx := SystemContext{
		OSRelease: "Fedora 40",
		CWD:       "/home/user",
	}

	req := BuildChatRequest("qwen3-coder:latest", sysCtx, "tar -czf archive.tar.gz dir", []string{"-czf"}, "tar summary")
	if len(req.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(req.Messages))
	}
	if len(req.Tools) < 2 {
		t.Fatalf("expected at least 2 Tools in ChatRequest, got %d", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "fetch-command-documentation" {
		t.Errorf("tool 0 name = %q, want 'fetch-command-documentation'", req.Tools[0].Function.Name)
	}
	if req.Tools[1].Function.Name != "command-run" {
		t.Errorf("tool 1 name = %q, want 'command-run'", req.Tools[1].Function.Name)
	}
}

func TestFormatToolResponse(t *testing.T) {
	cmdDoc := CommandDoc{
		Command:    "tar",
		HelpOutput: "Usage: tar [OPTION...] [FILE]...\n  -c, --create",
		ManOutput:  "TAR(1) - The GNU version of the tar archiving utility",
		Cheatsheets: []CheatsheetDoc{
			{
				Source:  "tldr",
				Name:    "tar.md",
				Content: "# tar\n> Archiving utility.\n> Create archive:\n`tar -czvf target.tar.gz /path/to/dir`",
			},
			{
				Source:  "cheat-sh",
				Name:    "tar",
				Content: "tar -xvf file.tar",
			},
		},
	}

	formatted := FormatToolResponse(cmdDoc)
	for _, want := range []string{
		"--- [tar] --help output ---",
		"-c, --create",
		"--- [tar] man page ---",
		"TAR(1)",
		"--- [tar] tldr cheatsheet (tar.md) ---",
		"tar -czvf target.tar.gz",
		"--- [tar] cheat-sh cheatsheet (tar) ---",
		"tar -xvf file.tar",
	} {
		if !strings.Contains(formatted, want) {
			t.Errorf("formatted tool response missing expected chunk %q\nFull output:\n%s", want, formatted)
		}
	}
}

func TestExtractToolCalls(t *testing.T) {
	// 1. Structured ToolCalls in Message
	msg1 := Message{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				Function: ToolCallFunction{
					Name:      "fetch-command-documentation",
					Arguments: map[string]any{"command": "ls"},
				},
			},
		},
	}
	calls1 := ExtractToolCalls(msg1)
	if len(calls1) != 1 || calls1[0].Function.Name != "fetch-command-documentation" {
		t.Errorf("calls1 = %+v, want 1 fetch-command-documentation call", calls1)
	}

	// 2. XML tag fallback: <tool-call>...</tool-call>
	msg2 := Message{
		Role:    "assistant",
		Content: `<tool-call>{"name": "fetch-command-documentation", "arguments": {"command": "tar", "subcommand": "create"}}</tool-call>`,
	}
	calls2 := ExtractToolCalls(msg2)
	if len(calls2) != 1 || calls2[0].Function.Name != "fetch-command-documentation" {
		t.Fatalf("calls2 = %+v, want 1 call", calls2)
	}
	if calls2[0].Function.Arguments["command"] != "tar" {
		t.Errorf("command = %v, want tar", calls2[0].Function.Arguments["command"])
	}

	// 3. Functional text fallback: fetch-command-documentation(command="git", subcommand="commit")
	msg3 := Message{
		Role:    "assistant",
		Content: `Let me check the documentation: fetch-command-documentation(command="git", subcommand="commit")`,
	}
	calls3 := ExtractToolCalls(msg3)
	if len(calls3) != 1 || calls3[0].Function.Name != "fetch-command-documentation" {
		t.Fatalf("calls3 = %+v, want 1 call", calls3)
	}
	if calls3[0].Function.Arguments["command"] != "git" || calls3[0].Function.Arguments["subcommand"] != "commit" {
		t.Errorf("args = %v, want git commit", calls3[0].Function.Arguments)
	}

	// 4. Functional text fallback: command-run(command="ls -la")
	msg4 := Message{
		Role:    "assistant",
		Content: `Let me check the files: command-run(command="ls -la")`,
	}
	calls4 := ExtractToolCalls(msg4)
	if len(calls4) != 1 || calls4[0].Function.Name != "command-run" {
		t.Fatalf("calls4 = %+v, want 1 call", calls4)
	}
	if calls4[0].Function.Arguments["command"] != "ls -la" {
		t.Errorf("args = %v, want ls -la", calls4[0].Function.Arguments)
	}
}

func TestBuildChatRequestDynamicNonceXMLBoundaries(t *testing.T) {
	sysCtx := SystemContext{
		OSRelease: "Fedora 43",
		CWD:       "/home/javi",
		RecentHistory: []string{
			"ls -javi # quiero diferenciar entre binarios y archivos de texto",
			"yups --test-models",
		},
		FileSnippets: map[string]string{
			"script.sh": "#!/bin/bash\necho test\n",
		},
	}

	req := BuildChatRequest("qwen2.5-coder:7b", sysCtx, "grep -r pattern .", []string{"-r"}, "grep summary")
	if len(req.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(req.Messages))
	}

	sysMsg := req.Messages[0].Content
	userMsg := req.Messages[1].Content

	// Check that system message contains security instructions, XML tags, and strict JSON format rules
	if !strings.Contains(sysMsg, "CRITICAL DATA INTEGRITY") {
		t.Errorf("sysMsg missing CRITICAL DATA INTEGRITY instruction:\n%s", sysMsg)
	}
	if !strings.Contains(sysMsg, "Final Response Format (STRICT JSON)") {
		t.Errorf("sysMsg missing Final Response Format (STRICT JSON) instruction:\n%s", sysMsg)
	}
	if !strings.Contains(sysMsg, `"suggested-command"`) {
		t.Errorf("sysMsg missing suggested-command schema in instructions:\n%s", sysMsg)
	}
	if !strings.Contains(sysMsg, "<system-context-") {
		t.Errorf("sysMsg missing <system-context- tag:\n%s", sysMsg)
	}
	if !strings.Contains(sysMsg, "<recent-shell-history-") {
		t.Errorf("sysMsg missing <recent-shell-history- tag:\n%s", sysMsg)
	}
	if !strings.Contains(sysMsg, "<history-entry>ls -javi # quiero diferenciar entre binarios y archivos de texto</history-entry>") {
		t.Errorf("sysMsg missing history entry:\n%s", sysMsg)
	}

	// Check that user message contains XML tags and strict JSON task instruction
	if !strings.Contains(userMsg, "<user-command-line-") || !strings.Contains(userMsg, "grep -r pattern .") {
		t.Errorf("userMsg missing <user-command-line- tag:\n%s", userMsg)
	}
	if !strings.Contains(userMsg, "<unknown-items-") || !strings.Contains(userMsg, "- -r") {
		t.Errorf("userMsg missing <unknown-items- tag:\n%s", userMsg)
	}
	if !strings.Contains(userMsg, "strict JSON object") {
		t.Errorf("userMsg missing strict JSON task requirement:\n%s", userMsg)
	}
}
