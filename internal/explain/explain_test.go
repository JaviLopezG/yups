package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yups/internal/llm"
)

type fakeFileInfo struct {
	isDir bool
	name  string
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

func TestExplainSimpleCommand(t *testing.T) {
	docEnv := DocEnv{
		TypeCmd: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls is an alias for `ls --color=auto'", nil
			}
			return "", nil
		},
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1)               - list directory contents", nil
			}
			return "", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if name == "ls" {
				return []byte(sampleLsHelp), nil
			}
			return nil, nil
		},
		StatPath: func(path string) (fs.FileInfo, error) {
			if path == "/var/cache/man/" {
				return fakeFileInfo{isDir: true, name: "man"}, nil
			}
			return nil, fs.ErrNotExist
		},
		LookupInPath: func(cmd string) bool {
			return cmd == "ls"
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-al", "/var/cache/man/"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	for _, want := range []string{
		"#_?",
		"Found: ls",
		"ls is an alias for `ls --color=auto'",
		"ls (1)               - list directory contents",
		"-a found:",
		"do not ignore entries starting with .",
		"-l found:",
		"use a long listing format",
		"/var/cache/man/ (directory)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainCompoundPipeline(t *testing.T) {
	docEnv := DocEnv{
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1) - list directory contents", nil
			}
			if cmd == "ps" {
				return "ps (1) - report a snapshot of the current processes", nil
			}
			return "", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if name == "ls" {
				return []byte(sampleLsHelp), nil
			}
			if name == "ps" {
				return []byte("  aux   BSD syntax options"), nil
			}
			return nil, nil
		},
		LookupInPath: func(cmd string) bool {
			return cmd == "ls" || cmd == "ps"
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-l", "||", "ps", "aux"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	for _, want := range []string{
		"#_?",
		"Found: ls",
		"ls (1) - list directory contents",
		"-l found:",
		"If the previous command fails (exit code != 0), executes:",
		"Found: ps",
		"ps (1) - report a snapshot of the current processes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainFallbackToMan(t *testing.T) {
	docEnv := DocEnv{
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "", nil // whatis unavailable
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return nil, nil // --help unavailable or failing
		},
		ManPage: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return sampleLsMan, nil
			}
			return "", nil
		},
		LookupInPath: func(cmd string) bool {
			return cmd == "ls"
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-l"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	for _, want := range []string{
		"#_?",
		"Found: ls",
		"ls - list directory contents",
		"-l found:",
		"use a long listing format",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainUnknownCommandCallsLLM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "'sl' is a joke program displaying a steam locomotive when you mistype 'ls'.\nSuggested command: ls -al",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"sl", "-al"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	for _, want := range []string{
		"No manual entry or help found for \"sl\"",
		"LLM Explanation:",
		"steam locomotive",
		"Suggested command:",
		"ls -al",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainUnreachableLLMShowsErrorAndInstallHint(t *testing.T) {
	// Point to a closed port
	client := llm.NewClient(&http.Client{Timeout: 100 * time.Millisecond}, "http://127.0.0.1:54321")
	docEnv := DocEnv{
		LLMClient:   client,
		IsInstalled: false,
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"sl", "-al"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	for _, want := range []string{
		"No manual entry or help found for \"sl\"",
		"Asking LLM (qwen2.5-coder:7b) at http://127.0.0.1:54321 for more information...",
		"Cannot connect to Ollama at http://127.0.0.1:54321",
		"Note: yups is using default settings because it is not installed or configured yet.",
		"Run 'yups --install-yups'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainInteractiveExecutionYes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Typo in command.\nSuggested command: ls -av",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	var executedCmd string
	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		AskPrompt: func(prompt, defaultValue string) string {
			return "y"
		},
		ExecShell: func(cmd string, stdout, stderr io.Writer) int {
			executedCmd = cmd
			return 42
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi"}, &stdout, &stderr, false)
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}
	if executedCmd != "ls -av" {
		t.Errorf("executedCmd = %q, want %q", executedCmd, "ls -av")
	}
}

func TestExplainInteractiveExecutionNo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Typo in command.\nSuggested command: ls -av",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	var executed bool
	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		AskPrompt: func(prompt, defaultValue string) string {
			return "n"
		},
		ExecShell: func(cmd string, stdout, stderr io.Writer) int {
			executed = true
			return 0
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if executed {
		t.Error("command should not have been executed on 'n'")
	}
}

func TestExplainInteractiveExecutionEdit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Typo in command.\nSuggested command: ls -av",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	prompts := []string{"e", "y"}
	promptIdx := 0

	var executedCmd string
	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		AskPrompt: func(prompt, defaultValue string) string {
			if promptIdx < len(prompts) {
				res := prompts[promptIdx]
				promptIdx++
				return res
			}
			return "n"
		},
		AskEditPrompt: func(prompt, initialValue string) string {
			return initialValue + " > output.txt"
		},
		ExecShell: func(cmd string, stdout, stderr io.Writer) int {
			executedCmd = cmd
			return 0
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if executedCmd != "ls -av > output.txt" {
		t.Errorf("executedCmd = %q, want %q", executedCmd, "ls -av > output.txt")
	}
}

func TestExplainInteractiveExecutionModifications(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		var resp llm.ChatResponse
		if callCount == 1 {
			resp = llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Initial suggestion.\nSuggested command: ls -av",
				},
				Done: true,
			}
		} else {
			// Verify multi-turn history includes user modifications
			lastMsg := chatReq.Messages[len(chatReq.Messages)-1]
			if !strings.Contains(lastMsg.Content, "recursive") {
				t.Errorf("last message does not contain 'recursive': %q", lastMsg.Content)
			}
			resp = llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Revised suggestion with recursion.\nSuggested command: ls -laR",
				},
				Done: true,
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	prompts := []string{"m", "I want recursive listing", "y"}
	promptIdx := 0

	var executedCmd string
	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		AskPrompt: func(prompt, defaultValue string) string {
			if promptIdx < len(prompts) {
				res := prompts[promptIdx]
				promptIdx++
				return res
			}
			return "n"
		},
		ExecShell: func(cmd string, stdout, stderr io.Writer) int {
			executedCmd = cmd
			return 0
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if executedCmd != "ls -laR" {
		t.Errorf("executedCmd = %q, want %q", executedCmd, "ls -laR")
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestTokenizeWithComment(t *testing.T) {
	tokens := Tokenize([]string{"ls", "-avi", "#Quiero listar subdirectorios"})
	if len(tokens) != 3 {
		t.Fatalf("len(tokens) = %d, want 3 (%+v)", len(tokens), tokens)
	}
	if tokens[0].Value != "ls" || tokens[1].Value != "-avi" {
		t.Errorf("unexpected command tokens: %+v", tokens[:2])
	}
	if tokens[2].Type != TokenComment || tokens[2].Value != "Quiero listar subdirectorios" {
		t.Errorf("unexpected comment token: %+v", tokens[2])
	}
}

func TestParseCommandWithComment(t *testing.T) {
	p := Parse([]string{"ls", "-a", "#", "buscar", "todo"})
	if len(p.Stages) != 1 {
		t.Fatalf("len(stages) = %d, want 1", len(p.Stages))
	}
	cmd := p.Stages[0].Command
	if cmd == nil {
		t.Fatal("expected command stage")
	}
	if cmd.Name != "ls" {
		t.Errorf("cmd.Name = %q, want 'ls'", cmd.Name)
	}
	if cmd.Comment != "buscar todo" {
		t.Errorf("cmd.Comment = %q, want 'buscar todo'", cmd.Comment)
	}
}

func TestExplainKnownCommandWithCommentTriggersLLM(t *testing.T) {
	llmCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalled = true
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		userMsg := chatReq.Messages[len(chatReq.Messages)-1].Content
		if !strings.Contains(userMsg, "#Quiero listar todos los subdirectorios") {
			t.Errorf("user message did not contain comment: %q", userMsg)
		}

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "To list all subdirectories recursively, use the -R flag.\nSuggested command: ls -laR",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1) - list directory contents", nil
			}
			return "", nil
		},
		LookupInPath: func(cmd string) bool { return cmd == "ls" },
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if name == "ls" {
				return []byte("  -a, --all    do not ignore entries starting with .\n"), nil
			}
			return nil, nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-a", "#Quiero listar todos los subdirectorios"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !llmCalled {
		t.Error("expected LLM to be queried when comment is present")
	}

	out := stdout.String()
	for _, want := range []string{
		"Found: ls",
		"-a found:",
		"# Quiero listar todos los subdirectorios",
		"Asking advanced LLM (" + llm.FallbackAdvancedModel + ") at " + ts.URL,
		"LLM Explanation:",
		"recursively",
		"Suggested command:",
		"ls -laR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainWithToolCallFetchesDocumentation(t *testing.T) {
	cheatsDir := t.TempDir()
	tldrDir := filepath.Join(cheatsDir, "tldr", "pages", "common")
	_ = os.MkdirAll(tldrDir, 0755)
	_ = os.WriteFile(filepath.Join(tldrDir, "ls.md"), []byte("# ls\n> List directory contents.\n> List all:\n`ls -a`"), 0644)

	var requestCount int
	var receivedToolMsg string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		if requestCount == 1 {
			// First turn: LLM requests documentation for 'ls'
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch_command_documentation",
								Arguments: map[string]any{"command": "ls"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Second turn: capture tool response and return final answer
		for _, m := range chatReq.Messages {
			if m.Role == "tool" {
				receivedToolMsg = m.Content
			}
		}

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Explanation based on cheatsheets.\nSuggested command: ls -a",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:      llm.NewClient(ts.Client(), ts.URL),
		CheatsheetsDir: cheatsDir,
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
		ManPage: func(ctx context.Context, cmd string) (string, error) {
			return "LS(1) manual page full text", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: ls [OPTION]...\n  -a, --all  list all"), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknownflag"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}

	for _, want := range []string{
		"--- [ls] --help output ---",
		"Usage: ls",
		"--- [ls] man page ---",
		"LS(1) manual page",
		"--- [ls] tldr cheatsheet (ls.md) ---",
		"List directory contents",
	} {
		if !strings.Contains(receivedToolMsg, want) {
			t.Errorf("Tool message missing expected section %q\nFull tool payload:\n%s", want, receivedToolMsg)
		}
	}

	out := stdout.String()
	for _, want := range []string{
		"LLM requested detailed documentation for 'ls'",
		"Gathering manual pages, --help, and cheatsheets for 'ls'",
		"Explanation based on cheatsheets.",
		"Suggested command:",
		"ls -a",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainCompoundPipelineWithMissingItemsQueriesEntirePipeline(t *testing.T) {
	var capturedUserPrompt string
	var capturedTools []llm.Tool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		if len(chatReq.Messages) > 1 {
			capturedUserPrompt = chatReq.Messages[1].Content
		}
		capturedTools = chatReq.Tools

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Both commands explained.\nSuggested command: ls -a && yups --help",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1) - list directory contents", nil
			}
			if cmd == "yups" {
				return "yups - yups CLI assistant", nil
			}
			return "", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if name == "ls" {
				return []byte("Usage: ls [OPTION]...\n  -a  all\n  -v  version"), nil
			}
			if name == "yups" {
				return []byte("Usage: yups [flags]\n  -V, --version  version\n  --help  help"), nil
			}
			return nil, nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi", "&&", "yups", "-hV"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	// Verify the user prompt sent to LLM contains the full pipeline
	if !strings.Contains(capturedUserPrompt, "User command line: ls -javi && yups -hV") {
		t.Errorf("prompt does not contain full command line: %s", capturedUserPrompt)
	}

	// Verify summary includes both commands
	if !strings.Contains(capturedUserPrompt, "Command 1 [ls]:") || !strings.Contains(capturedUserPrompt, "Command 2 [yups]:") {
		t.Errorf("prompt missing command 1 or 2 in basic summary:\n%s", capturedUserPrompt)
	}

	// Verify missing items lists unknown options for both
	if !strings.Contains(capturedUserPrompt, "[ls] Option \"-j\"") || !strings.Contains(capturedUserPrompt, "[yups] Option \"-h\"") {
		t.Errorf("prompt missing specific missing items for both commands:\n%s", capturedUserPrompt)
	}

	// Verify tools were provided
	if len(capturedTools) == 0 || capturedTools[0].Function.Name != "fetch_command_documentation" {
		t.Errorf("expected tools in request, got %+v", capturedTools)
	}

	out := stdout.String()
	if !strings.Contains(out, "Suggested command:\n  ls -a && yups --help") {
		t.Errorf("stdout does not contain full corrected pipeline suggestion:\n%s", out)
	}
}

func TestExplainPipelineWithOrAndNotFoundCommand(t *testing.T) {
	var capturedUserPrompt string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)
		if len(chatReq.Messages) > 1 {
			capturedUserPrompt = chatReq.Messages[1].Content
		}

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Explanation for both ls and tree.\nSuggested command: ls -a || tree",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1) - list directory contents", nil
			}
			return "", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if name == "ls" {
				return []byte("Usage: ls [OPTION]...\n  -a  all"), nil
			}
			return nil, nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls -javi || tree"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if !strings.Contains(capturedUserPrompt, "User command line: ls -javi || tree") {
		t.Errorf("prompt does not contain full command line: %s", capturedUserPrompt)
	}
	if !strings.Contains(capturedUserPrompt, "Command \"tree\" was not found in system PATH") {
		t.Errorf("prompt missing tree not found error:\n%s", capturedUserPrompt)
	}
	if !strings.Contains(capturedUserPrompt, "|| (If the previous command fails") {
		t.Errorf("prompt missing || operator explanation:\n%s", capturedUserPrompt)
	}
}

func TestExplainMultiTurnToolCalls(t *testing.T) {
	var requestCount int
	var toolsProvidedInTurn2 bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		if requestCount == 1 {
			// Turn 1: request documentation for 'ls'
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch_command_documentation",
								Arguments: map[string]any{"command": "ls"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if requestCount == 2 {
			// Turn 2: verify tools are still provided, and request documentation for 'yups'
			if len(chatReq.Tools) > 0 {
				toolsProvidedInTurn2 = true
			}
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch_command_documentation",
								Arguments: map[string]any{"command": "yups"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 3: final answer
		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Option -j does not exist for ls. For yups, use --help.\nSuggested command: ls -avi && yups --help",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			if cmd == "ls" {
				return "ls (1) - list directory contents", nil
			}
			if cmd == "yups" {
				return "yups - yups assistant", nil
			}
			return "", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: " + name + " [options]"), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-javi", "&&", "yups", "-hV"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestCount != 3 {
		t.Errorf("requestCount = %d, want 3", requestCount)
	}
	if !toolsProvidedInTurn2 {
		t.Errorf("tools were not provided in turn 2 request")
	}

	out := stdout.String()
	for _, want := range []string{
		"LLM requested detailed documentation for 'ls'",
		"LLM requested detailed documentation for 'yups'",
		"Suggested command:\n  ls -avi && yups --help",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainWithCommandRunToolExecutesAndReturnsOutput(t *testing.T) {
	var requestCount int
	var receivedToolMsg string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		if requestCount == 1 {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "ls -l | grep test"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		for _, m := range chatReq.Messages {
			if m.Role == "tool" {
				receivedToolMsg = m.Content
			}
		}

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Found matching files.\nSuggested command: ls -la",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "-c" {
				return []byte("-rw-r--r-- 1 user user 123 test.txt"), nil
			}
			return []byte("Usage: ls [OPTION]..."), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}

	if !strings.Contains(receivedToolMsg, "test.txt") {
		t.Errorf("expected command output in tool message, got %q", receivedToolMsg)
	}

	out := stdout.String()
	for _, want := range []string{
		"LLM requested running command: 'ls -l | grep test'",
		"Executing whitelisted command...",
		"Suggested command:\n  ls -la",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainWithCommandRunToolRejectsDisallowedCommand(t *testing.T) {
	var requestCount int
	var receivedToolMsg string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		if requestCount == 1 {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "rm -rf /tmp/junk"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		for _, m := range chatReq.Messages {
			if m.Role == "tool" {
				receivedToolMsg = m.Content
			}
		}

		resp := llm.ChatResponse{
			Model: "qwen2.5-coder:7b",
			Message: llm.Message{
				Role:    "assistant",
				Content: "Command was rejected.\nSuggested command: ls -a",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: ls [OPTION]..."), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if !strings.Contains(receivedToolMsg, "rejected by whitelist") {
		t.Errorf("expected whitelist rejection in tool message, got %q", receivedToolMsg)
	}

	out := stdout.String()
	if !strings.Contains(out, "blocked: command \"rm\" is not in the whitelist") {
		t.Errorf("stdout missing whitelist blocked warning:\n%s", out)
	}
}

func TestExplainCommentQuestionWithMultiTurnInspection(t *testing.T) {
	var turn int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)

		switch turn {
		case 1:
			// Turn 1: request grep /etc/hosts
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "grep trillian /etc/hosts"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)

		case 2:
			// Turn 2: request nslookup trillian
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "nslookup trillian.ts"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)

		case 3:
			// Turn 3: final answer with no command suggestion
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "La dirección IP de trillian es 100.64.0.42 según resolución DNS.",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && strings.Contains(args[1], "hosts") {
				return []byte(""), nil
			}
			if len(args) >= 2 && strings.Contains(args[1], "nslookup") {
				return []byte("Server: 100.100.100.100\nAddress: 100.64.0.42"), nil
			}
			return nil, nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"# ¿Cual es la ip de trillian?"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if turn != 3 {
		t.Errorf("turn = %d, want 3", turn)
	}

	out := stdout.String()
	for _, want := range []string{
		"LLM requested running command: 'grep trillian /etc/hosts'",
		"LLM requested running command: 'nslookup trillian.ts'",
		"LLM Explanation:",
		"La dirección IP de trillian es 100.64.0.42",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestExplainEscalatesToAdvancedModelOnToolCall(t *testing.T) {
	var requestedModels []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)
		requestedModels = append(requestedModels, chatReq.Model)

		if len(requestedModels) == 1 {
			// Turn 1 with default model: request tool
			resp := llm.ChatResponse{
				Model: chatReq.Model,
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch_command_documentation",
								Arguments: map[string]any{"command": "ls"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 2 with escalated model: final answer
		resp := llm.ChatResponse{
			Model: chatReq.Model,
			Message: llm.Message{
				Role:    "assistant",
				Content: "Explanation by advanced model.\nSuggested command: ls -la",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:  "qwen2.5-coder:7b",
		AdvancedModel: "gemma3:latest",
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: ls [OPTION]..."), nil
		},
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if len(requestedModels) != 2 {
		t.Fatalf("len(requestedModels) = %d, want 2", len(requestedModels))
	}
	if requestedModels[0] != "qwen2.5-coder:7b" {
		t.Errorf("turn 1 model = %q, want qwen2.5-coder:7b", requestedModels[0])
	}
	if requestedModels[1] != "gemma3:latest" {
		t.Errorf("turn 2 model = %q, want gemma3:latest", requestedModels[1])
	}

	out := stdout.String()
	if !strings.Contains(out, "Escalating to advanced model (gemma3:latest)") {
		t.Errorf("stdout missing escalation notice:\n%s", out)
	}
}

func TestExplainCommentUsesAdvancedModelFromStart(t *testing.T) {
	var capturedModel string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)
		capturedModel = chatReq.Model

		resp := llm.ChatResponse{
			Model: chatReq.Model,
			Message: llm.Message{
				Role:    "assistant",
				Content: "Answer by advanced model.",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:  "qwen2.5-coder:7b",
		AdvancedModel: "gemma3:latest",
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"# ¿Cómo listo archivos recursivamente?"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if capturedModel != "gemma3:latest" {
		t.Errorf("captured model = %q, want gemma3:latest", capturedModel)
	}

	out := stdout.String()
	if !strings.Contains(out, "Asking advanced LLM (gemma3:latest) at") {
		t.Errorf("stdout missing advanced notice:\n%s", out)
	}
}

func TestExplainOverrideModelUsesSpecifiedModelWithoutEscalation(t *testing.T) {
	var requestedModels []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&chatReq)
		requestedModels = append(requestedModels, chatReq.Model)

		if len(requestedModels) == 1 {
			resp := llm.ChatResponse{
				Model: chatReq.Model,
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch_command_documentation",
								Arguments: map[string]any{"command": "ls"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := llm.ChatResponse{
			Model: chatReq.Model,
			Message: llm.Message{
				Role:    "assistant",
				Content: "Done.\nSuggested command: ls -la",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:  "qwen2.5-coder:7b",
		AdvancedModel: "gemma3:latest",
		OverrideModel: "my-pinned-model:v1",
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: ls"), nil
		},
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	for i, m := range requestedModels {
		if m != "my-pinned-model:v1" {
			t.Errorf("turn %d model = %q, want my-pinned-model:v1", i+1, m)
		}
	}

	out := stdout.String()
	if strings.Contains(out, "Escalating to advanced model") {
		t.Errorf("expected no escalation when OverrideModel is set, got:\n%s", out)
	}
}

func TestExplainUsesAdvancedModelIfLoadedInMemoryViaPS(t *testing.T) {
	var requestedModel string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ps" {
			resp := map[string]any{
				"models": []map[string]any{
					{"name": "qwen3.8:latest", "model": "qwen3.8:latest", "size_vram": 5000000000},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/chat" {
			var req llm.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			requestedModel = req.Model
			resp := llm.ChatResponse{
				Model: req.Model,
				Message: llm.Message{
					Role:    "assistant",
					Content: "Done.\nSuggested command: ls -la",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:  "qwen2.5-coder:7b",
		AdvancedModel: "qwen3.8:latest",
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("Usage: ls"), nil
		},
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestedModel != "qwen3.8:latest" {
		t.Errorf("requestedModel = %q, want qwen3.8:latest because it was loaded in /api/ps", requestedModel)
	}

	out := stdout.String()
	if !strings.Contains(out, "Asking advanced LLM (qwen3.8:latest)") {
		t.Errorf("stdout missing advanced notice when loaded in memory:\n%s", out)
	}
}
