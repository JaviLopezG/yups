package explain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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
		"Asking LLM at http://127.0.0.1:54321 for more information...",
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

	prompts := []string{"e", "ls -lav /var/log", "y"}
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
	if executedCmd != "ls -lav /var/log" {
		t.Errorf("executedCmd = %q, want %q", executedCmd, "ls -lav /var/log")
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
