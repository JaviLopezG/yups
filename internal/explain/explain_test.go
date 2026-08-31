package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		"Asking LLM (" + llm.FallbackDefaultModel + ") at http://127.0.0.1:54321 for more information...",
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
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
	p := Parse([]string{"ls", "-avi", "#Quiero listar subdirectorios"})
	if len(p.Stages) != 1 {
		t.Fatalf("len(stages) = %d, want 1 (%+v)", len(p.Stages), p.Stages)
	}
	if p.Stages[0].Command.Name != "ls" {
		t.Errorf("unexpected command: %q, want 'ls'", p.Stages[0].Command.Name)
	}
	if p.Comment != "Quiero listar subdirectorios" {
		t.Errorf("unexpected comment: %q, want 'Quiero listar subdirectorios'", p.Comment)
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
		if !strings.Contains(userMsg, "# Quiero listar todos los subdirectorios") && !strings.Contains(userMsg, "#Quiero listar todos los subdirectorios") {
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
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
								Name:      "fetch-command-documentation",
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
	if !strings.Contains(capturedUserPrompt, "ls -javi && yups -hV") {
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
	if len(capturedTools) == 0 || capturedTools[0].Function.Name != "fetch-command-documentation" {
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

	if !strings.Contains(capturedUserPrompt, "ls -javi || tree") {
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
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
								Name:      "fetch-command-documentation",
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
			// Verify last message is a user message to satisfy Qwen Jinja templates
			lastMsg := chatReq.Messages[len(chatReq.Messages)-1]
			if lastMsg.Role != "user" {
				t.Errorf("expected last message in turn 2 to have role 'user', got %q", lastMsg.Role)
			}
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch-command-documentation",
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
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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

func TestExplainWithCommandRunToolTimeout(t *testing.T) {
	var requestCount int
	var receivedToolMsg string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
								Arguments: map[string]any{"command": "find . -type f -exec file {} +"},
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
				Content: "Command timed out, so here is the general explanation.\nSuggested command: find . -maxdepth 2",
			},
			Done: true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "find (1) - search for files in a directory hierarchy", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "-c" {
				// Simulate command timeout
				return nil, context.DeadlineExceeded
			}
			return []byte("Usage: find [path...] [expression]"), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"find", "-unknownflag"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}

	if !strings.Contains(receivedToolMsg, "timed out after") {
		t.Errorf("expected timeout notice in tool message, got %q", receivedToolMsg)
	}

	out := stdout.String()
	if !strings.Contains(out, "timed out after") {
		t.Errorf("stdout missing timeout notice:\n%s", out)
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

func TestExplainResponseWithSuggestionAndToolCallIsTreatedAsFinal(t *testing.T) {
	var requestCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
		requestCount++

		if requestCount == 1 {
			// Turn 1: model returns tool call for documentation and command-run
			resp := llm.ChatResponse{
				Model: "qwen3-coder:latest",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "fetch-command-documentation",
								Arguments: map[string]any{"command": "ls"},
							},
						},
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "ls -jal"},
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
			// Turn 2: model provides both content with Suggested command AND a tool_call
			resp := llm.ChatResponse{
				Model: "qwen3-coder:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "The option \"-j\" is not valid for ls.\n\nSuggested command: ls -al\nExplanation: The command contains an invalid option \"-j\".",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
								Arguments: map[string]any{"command": "ls -al"},
							},
						},
					},
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		t.Fatalf("unexpected turn %d: should have stopped after turn 2", requestCount)
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient: llm.NewClient(ts.Client(), ts.URL),
		Whatis: func(ctx context.Context, cmd string) (string, error) {
			return "ls (1) - list directory contents", nil
		},
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[0] == "-c" {
				return []byte("ls: invalid option -- 'j'"), nil
			}
			return []byte("Usage: ls [OPTION]..."), nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-jal"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if requestCount != 2 {
		t.Errorf("requestCount = %d, want exactly 2", requestCount)
	}

	out := stdout.String()
	if !strings.Contains(out, "Suggested command:\n  ls -al") {
		t.Errorf("stdout missing suggested command:\n%s", out)
	}
	if !strings.Contains(out, "The command contains an invalid option \"-j\"") {
		t.Errorf("stdout missing explanation:\n%s", out)
	}
}

func TestExplainKeepsDefaultModelOnToolCall(t *testing.T) {
	var requestedModels []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ps" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
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
								Name:      "fetch-command-documentation",
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

		// Turn 2 continues with default model: final answer
		resp := llm.ChatResponse{
			Model: chatReq.Model,
			Message: llm.Message{
				Role:    "assistant",
				Content: "Explanation by default model.\nSuggested command: ls -la",
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
	if requestedModels[1] != "qwen2.5-coder:7b" {
		t.Errorf("turn 2 model = %q, want qwen2.5-coder:7b", requestedModels[1])
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
								Name:      "fetch-command-documentation",
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

func TestLogoFlagsFormatting(t *testing.T) {
	tests := []struct {
		name      string
		flags     string
		rawCmd    string
		color     bool
		wantMatch string
	}{
		{
			name:      "color with flags and command",
			flags:     "--advanced --",
			rawCmd:    "ls -hal",
			color:     true,
			wantMatch: "\x1b[38;5;214m#_?\x1b[0m \x1b[1;36m--advanced --\x1b[0m \x1b[90mls -hal\x1b[0m",
		},
		{
			name:      "no color with flags and command",
			flags:     "--advanced --",
			rawCmd:    "ls -hal",
			color:     false,
			wantMatch: "#_? --advanced -- ls -hal",
		},
		{
			name:      "no flags with command",
			flags:     "",
			rawCmd:    "ls -hal",
			color:     false,
			wantMatch: "#_? ls -hal",
		},
		{
			name:      "color no flags with command",
			flags:     "",
			rawCmd:    "ls -hal",
			color:     true,
			wantMatch: "\x1b[38;5;214m#_?\x1b[0m \x1b[90mls -hal\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PipelineExplanation{
				InvocationFlags: tt.flags,
				RawCommandLine:  tt.rawCmd,
			}
			var buf bytes.Buffer
			FormatBasicPipeline(&buf, p, FormatOptions{Color: tt.color})
			got := strings.TrimSpace(buf.String())
			if !strings.Contains(got, tt.wantMatch) {
				t.Errorf("got %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}

func TestTurnLimitReachedInformsAndAbortsOnPrompt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			// Always return tool call to exhaust turns
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							Function: llm.ToolCallFunction{
								Name:      "command-run",
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
	}))
	defer ts.Close()

	var askedPrompt string
	docEnv := DocEnv{
		LLMClient:    llm.NewClient(ts.Client(), ts.URL),
		DefaultModel: "qwen2.5-coder:7b",
		MaxToolTurns: 2,
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("total 0\n"), nil
		},
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			askedPrompt = prompt
			return true // User chooses to abort
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "maximum reasoning tool rounds reached (2 turns)") {
		t.Errorf("expected turn limit message, got:\n%s", out)
	}
	if !strings.Contains(out, "Execution aborted.") {
		t.Errorf("expected aborted message, got:\n%s", out)
	}
	if askedPrompt != "Do you want to abort execution?" {
		t.Errorf("expected confirmation prompt 'Do you want to abort execution?', got %q", askedPrompt)
	}
}

func TestTurnLimitReachedSwitchesToAdvancedModelWhenContinuing(t *testing.T) {
	var chatModels []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			var req llm.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			chatModels = append(chatModels, req.Model)

			if len(chatModels) <= 2 {
				// Tool call in turns 1 and 2
				resp := llm.ChatResponse{
					Model: req.Model,
					Message: llm.Message{
						Role: "assistant",
						ToolCalls: []llm.ToolCall{
							{
								Function: llm.ToolCallFunction{
									Name:      "command-run",
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

			// Turn 3 (after user says no to abort): final answer with advanced model
			resp := llm.ChatResponse{
				Model: req.Model,
				Message: llm.Message{
					Role:    "assistant",
					Content: "Done after advanced switch.\nSuggested command: ls -la",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:          llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:       "qwen2.5-coder:7b",
		AdvancedModel:      "qwen3.8:latest",
		MaxToolTurns:       2,
		AdvancedMultiplier: 2,
		RunCmdTimeout: func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
			return []byte("total 0\n"), nil
		},
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			return false // User chooses NOT to abort
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "maximum reasoning tool rounds reached (2 turns)") {
		t.Errorf("expected turn limit message, got:\n%s", out)
	}
	if !strings.Contains(out, "Ctrl+C or Ctrl+Z") {
		t.Errorf("expected Ctrl+C / Ctrl+Z notice, got:\n%s", out)
	}
	if !strings.Contains(out, "Switching to advanced reasoning model (qwen3.8:latest)") {
		t.Errorf("expected switch to advanced model message, got:\n%s", out)
	}
	if !strings.Contains(out, "Suggested command:") || !strings.Contains(out, "ls -la") {
		t.Errorf("expected final suggested command, got:\n%s", out)
	}

	// Verify chat models escalated from default to advanced
	if len(chatModels) < 3 {
		t.Fatalf("expected at least 3 chat calls, got %d (%v)", len(chatModels), chatModels)
	}
	if chatModels[0] != "qwen2.5-coder:7b" {
		t.Errorf("turn 1 model = %q, want qwen2.5-coder:7b", chatModels[0])
	}
	if chatModels[len(chatModels)-1] != "qwen3.8:latest" {
		t.Errorf("final turn model = %q, want qwen3.8:latest", chatModels[len(chatModels)-1])
	}
}

func TestTimeoutReachedInformsAndAbortsOrContinues(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			callCount++
			if callCount == 1 {
				// Simulate slow query causing client timeout
				time.Sleep(100 * time.Millisecond)
			}
			resp := llm.ChatResponse{
				Model: "qwen3.8:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Success after retry.\nSuggested command: ls",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer ts.Close()

	docEnv := DocEnv{
		LLMClient:          llm.NewClient(ts.Client(), ts.URL),
		DefaultModel:       "qwen2.5-coder:7b",
		AdvancedModel:      "qwen3.8:latest",
		LLMTimeout:         20 * time.Millisecond,
		AdvancedMultiplier: 10,
		AskConfirmation: func(prompt string, defaultYes bool) bool {
			return false // Do not abort
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"ls", "-unknown"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	out := stdout.String()
	if !strings.Contains(out, "Execution limit reached: query taking longer than") {
		t.Errorf("expected timeout limit message, got:\n%s", out)
	}
	if !strings.Contains(out, "Ctrl+C or Ctrl+Z") {
		t.Errorf("expected Ctrl+C notice, got:\n%s", out)
	}
	if callCount != 1 {
		t.Errorf("expected callCount = 1 (continuous background request without restart), got %d", callCount)
	}
}

func TestFormatSuggestedScriptShort(t *testing.T) {
	script := "echo 1\necho 2\necho 3"
	var buf bytes.Buffer
	FormatSuggestedScript(&buf, script, FormatOptions{Color: false})

	out := buf.String()
	if !strings.Contains(out, "Suggested script:") {
		t.Errorf("missing header: %s", out)
	}
	if !strings.Contains(out, " 1 | echo 1") || !strings.Contains(out, " 2 | echo 2") || !strings.Contains(out, " 3 | echo 3") {
		t.Errorf("missing numbered lines: %s", out)
	}
	if strings.Contains(out, "(...)") {
		t.Errorf("short script should not be truncated:\n%s", out)
	}
}

func TestFormatSuggestedScriptLongTruncated(t *testing.T) {
	t.Setenv("LINES", "24") // 24 - 10 = 14 lines threshold
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line_%02d", i))
	}
	script := strings.Join(lines, "\n")

	var buf bytes.Buffer
	FormatSuggestedScript(&buf, script, FormatOptions{Color: false})

	out := buf.String()
	if !strings.Contains(out, " 1 | line_01") || !strings.Contains(out, " 5 | line_05") {
		t.Errorf("missing head lines: %s", out)
	}
	if !strings.Contains(out, "(...)") || !strings.Contains(out, "-----------------------") {
		t.Errorf("missing truncation indicator: %s", out)
	}
	if !strings.Contains(out, "16 | line_16") || !strings.Contains(out, "20 | line_20") {
		t.Errorf("missing tail lines: %s", out)
	}
	if strings.Contains(out, "line_10") {
		t.Errorf("line_10 should be hidden by truncation:\n%s", out)
	}
}

func TestFormatLLMPipelineResultScriptBeforeCommand(t *testing.T) {
	exp := &PipelineExplanation{
		LLMQueried:       true,
		LLMExplanation:   "This does something.",
		SuggestedCommand: "echo 'hello cmd'",
		SuggestedScript:  "echo 'hello script'",
	}

	var buf bytes.Buffer
	FormatLLMPipelineResult(&buf, exp, FormatOptions{Color: false})
	out := buf.String()

	scriptIdx := strings.Index(out, "Suggested script:")
	cmdIdx := strings.Index(out, "Suggested command:")

	if scriptIdx == -1 || cmdIdx == -1 {
		t.Fatalf("expected both script and command in output:\n%s", out)
	}
	if scriptIdx >= cmdIdx {
		t.Errorf("expected script to appear before command, scriptIdx=%d, cmdIdx=%d\n%s", scriptIdx, cmdIdx, out)
	}
}

func TestExplainDualSuggestionFlow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Dual suggestion:\n```json\n{\n  \"explanation\": \"Both cmd and script\",\n  \"suggested-command\": \"echo run-cmd\",\n  \"suggested-script\": \"echo run-script-line1\\necho run-script-line2\"\n}\n```",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	var promptsAsked []string
	var executedCmd string

	tempScriptsDir := t.TempDir()
	docEnv := DocEnv{
		LLMClient:   llm.NewClient(ts.Client(), ts.URL),
		UseAdvanced: true,
		ScriptsDir:  tempScriptsDir,
		AskPrompt: func(prompt, defaultValue string) string {
			promptsAsked = append(promptsAsked, prompt)
			if strings.Contains(prompt, "script") {
				return "no" // decline script
			}
			return "yes" // accept command
		},
		ExecShell: func(command string, stdout, stderr io.Writer) int {
			executedCmd = command
			return 0
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"my-unknown-command"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if len(promptsAsked) != 2 {
		t.Fatalf("expected 2 prompts (script then command), got %d: %v", len(promptsAsked), promptsAsked)
	}
	if !strings.Contains(promptsAsked[0], "script") {
		t.Errorf("first prompt should be for script: %q", promptsAsked[0])
	}
	if !strings.Contains(promptsAsked[1], "command") {
		t.Errorf("second prompt should be for command: %q", promptsAsked[1])
	}
	if executedCmd != "echo run-cmd" {
		t.Errorf("executed command = %q, want 'echo run-cmd'", executedCmd)
	}
}

func TestExplainScriptEditingFlow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "qwen2.5-coder:7b",
				Message: llm.Message{
					Role:    "assistant",
					Content: "```json\n{\n  \"explanation\": \"Only script\",\n  \"suggested-script\": \"echo test-script\\necho line2\"\n}\n```",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	tempScriptsDir := t.TempDir()
	var editorOpenedFile string
	prompts := []string{"edit", "no"}
	promptIdx := 0

	docEnv := DocEnv{
		LLMClient:   llm.NewClient(ts.Client(), ts.URL),
		UseAdvanced: true,
		ScriptsDir:  tempScriptsDir,
		AskPrompt: func(prompt, defaultValue string) string {
			if promptIdx < len(prompts) {
				res := prompts[promptIdx]
				promptIdx++
				return res
			}
			return "no"
		},
		OpenEditor: func(path string, stdin io.Reader, stdout, stderr io.Writer) error {
			editorOpenedFile = path
			return nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"my-unknown-command"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if editorOpenedFile == "" {
		t.Fatal("OpenEditor was not called")
	}
	if !strings.HasPrefix(editorOpenedFile, tempScriptsDir) {
		t.Errorf("editor opened file %q outside scriptsDir %q", editorOpenedFile, tempScriptsDir)
	}
	if !strings.HasSuffix(editorOpenedFile, ".sh") {
		t.Errorf("editor opened file %q without .sh extension", editorOpenedFile)
	}
	if !strings.Contains(stdout.String(), "Script saved to "+editorOpenedFile+" (available as $YUPS_SCRIPT)") {
		t.Errorf("missing saved notice in stdout:\n%s", stdout.String())
	}
	if os.Getenv("YUPS_SCRIPT") != editorOpenedFile {
		t.Errorf("YUPS_SCRIPT env = %q, want %q", os.Getenv("YUPS_SCRIPT"), editorOpenedFile)
	}
}

func TestExplainEmptyResponseRetriedAndSucceeds(t *testing.T) {
	callCount := 0
	retrySawReminder := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			callCount++
			var req llm.ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if callCount == 1 {
				// Turn 1 returns empty content
				resp := llm.ChatResponse{
					Model: "qwen3-coder:latest",
					Message: llm.Message{
						Role:    "assistant",
						Content: "",
					},
					Done: true,
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			// Turn 2: verify reminder was included in conversation
			for _, m := range req.Messages {
				if strings.Contains(m.Content, "Your previous response was completely empty") {
					retrySawReminder = true
				}
			}
			resp := llm.ChatResponse{
				Model: "qwen3-coder:latest",
				Message: llm.Message{
					Role:    "assistant",
					Content: "Now it works.\nSuggested command: ls",
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
		AdvancedModel: "test-model",
		UseAdvanced:   true,
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"my-unknown-cmd"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (1 empty + 1 retry)", callCount)
	}
	if !retrySawReminder {
		t.Errorf("expected retry request to include reminder message")
	}
	if !strings.Contains(stdout.String(), "Suggested command:\n  ls") {
		t.Errorf("expected successful command suggestion in stdout, got:\n%s", stdout.String())
	}
}

func TestExplainConsecutiveEmptyResponsesWarnsAndAborts(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			callCount++
			resp := llm.ChatResponse{
				Model: "test-model",
				Message: llm.Message{
					Role:    "assistant",
					Content: "",
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
		AdvancedModel: "test-model",
		UseAdvanced:   true,
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"my-unknown-cmd"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
	if !strings.Contains(stdout.String(), "Warning: The model test-model returned an empty response.") {
		t.Errorf("expected empty response warning in stdout, got:\n%s", stdout.String())
	}
}

func TestExplainContextCancellationInterruptsAndReturns130(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			// Simulate long-running request
			time.Sleep(500 * time.Millisecond)
			resp := llm.ChatResponse{
				Model: "test-model",
				Message: llm.Message{
					Role:    "assistant",
					Content: "done",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel context after 50ms while request is in flight
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		AdvancedModel: "test-model",
		UseAdvanced:   true,
	}

	var stdout, stderr bytes.Buffer
	code := Explain(ctx, docEnv, []string{"my-unknown-cmd"}, &stdout, &stderr, false)
	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
}

func TestExplainFlushesStdinBeforeInteractivePrompt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			resp := llm.ChatResponse{
				Model: "test-model",
				Message: llm.Message{
					Role:    "assistant",
					Content: "```json\n{\n  \"explanation\": \"test\",\n  \"suggested-command\": \"ls -la\"\n}\n```",
				},
				Done: true,
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer ts.Close()

	flushCalled := false
	docEnv := DocEnv{
		LLMClient:     llm.NewClient(ts.Client(), ts.URL),
		AdvancedModel: "test-model",
		UseAdvanced:   true,
		FlushStdin: func() {
			flushCalled = true
		},
		AskPrompt: func(prompt, defaultValue string) string {
			return "n"
		},
	}

	var stdout, stderr bytes.Buffer
	code := Explain(context.Background(), docEnv, []string{"my-unknown-cmd"}, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !flushCalled {
		t.Errorf("expected FlushStdin to be called before interactive prompt")
	}
}
