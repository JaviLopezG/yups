package sessionlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yups/internal/llm"
)

func TestSessionLoggerRecordsTraceAndInteractions(t *testing.T) {
	tempHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempHome, ".yups"), 0o755)
	args := []string{"--advanced", "--", "ls", "-la"}

	logger := New(tempHome, args)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.LogConfig("http://localhost:11434", "qwen2.5-coder:latest", "qwen3.8:latest", true, 60*time.Second, 30*time.Second, 10, 4096, 3)
	logger.LogModelResolution("qwen3.8:latest", true, "--advanced flag provided")

	chatReq := llm.ChatRequest{
		Model: "qwen3.8:latest",
		Messages: []llm.Message{
			{Role: "user", Content: "explain ls -la"},
		},
	}
	chatResp := llm.ChatResponse{
		Model: "qwen3.8:latest",
		Message: llm.Message{
			Role:    "assistant",
			Content: "ls -la lists directory contents in long format.",
		},
		Done: true,
	}

	logger.LogInteraction(0, "qwen3.8:latest", "http://localhost:11434", chatReq, chatResp, 150*time.Millisecond, nil)
	logger.LogToolExecution(0, "command-run", map[string]any{"command": "ls"}, true, "whitelisted", "total 0\n", 20*time.Millisecond, nil)
	logger.LogLimitReached("max-turns", "reached 10 turns", false)
	logger.LogConclusion("ls -la lists directory contents", "ls -la", "", "SUCCESS", 0)

	bufStr := logger.BufferString()
	for _, want := range []string{
		"YUPS SESSION LOG:",
		"Command:     yups --advanced -- ls -la",
		"Inference Endpoint:       http://localhost:11434",
		"Selected Model: qwen3.8:latest",
		">>> [OLLAMA INTERACTION Turn 0] >>>",
		"Request Payload:",
		"explain ls -la",
		"ls -la lists directory contents",
		"--- [TOOL EXECUTION Turn 0: command-run] ---",
		"[! LIMIT REACHED: max-turns !]",
		"SESSION CONCLUSION",
		"Status:            SUCCESS",
	} {
		if !strings.Contains(bufStr, want) {
			t.Errorf("buffer missing %q", want)
		}
	}

	// Verify log file was written to ~/.yups/logs/
	logPath := logger.LogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading detailed log file: %v", err)
	}
	if !strings.Contains(string(data), "YUPS SESSION LOG:") {
		t.Error("detailed log file missing header")
	}

	// Verify summary log was appended
	summaryPath := filepath.Join(tempHome, ".yups", "logs", "sessions.log")
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("reading sessions.log: %v", err)
	}
	if !strings.Contains(string(summaryData), "yups --advanced -- ls -la") {
		t.Errorf("sessions.log missing summary entry: %s", string(summaryData))
	}
}

func TestSessionLoggerDisabledOnEmptyHome(t *testing.T) {
	logger := New("", nil)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	logger.LogInfo("test info")
	logger.LogConclusion("", "", "", "OK", 0)
	if logger.LogPath() != "" {
		t.Errorf("expected empty LogPath, got %q", logger.LogPath())
	}
}

func TestSessionLoggerDoesNotCreateLogsWhenStateDirMissing(t *testing.T) {
	tempHome := t.TempDir()
	// Do NOT create ~/.yups
	logger := New(tempHome, []string{"--help"})
	logger.LogInfo("help displayed")
	logger.LogConclusion("", "", "", "HELP", 0)

	stateDir := filepath.Join(tempHome, ".yups")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected stateDir %s to not exist, but it was created", stateDir)
	}
}
