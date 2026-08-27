package explain

import (
	"bytes"
	"context"
	"encoding/json"
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
