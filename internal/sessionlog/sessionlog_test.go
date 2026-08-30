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

	// Verify individual request/response log was written to ~/.yups/logs/llm-requests/session-.../
	reqDir := logger.RequestsDir()
	reqFiles, err := os.ReadDir(reqDir)
	if err != nil {
		t.Fatalf("reading requestsDir %s: %v", reqDir, err)
	}
	if len(reqFiles) != 1 {
		t.Fatalf("expected 1 request log file in %s, got %d", reqDir, len(reqFiles))
	}
	if !strings.HasPrefix(reqFiles[0].Name(), "request-") || !strings.HasSuffix(reqFiles[0].Name(), "-turn-0.log") {
		t.Errorf("unexpected request log filename: %q", reqFiles[0].Name())
	}

	reqContent, err := os.ReadFile(filepath.Join(reqDir, reqFiles[0].Name()))
	if err != nil {
		t.Fatalf("reading request log file: %v", err)
	}
	reqStr := string(reqContent)
	for _, want := range []string{
		"YUPS LLM REQUEST / RESPONSE INTERACTION",
		"Turn:        0",
		"Model:       qwen3.8:latest",
		"REQUEST PAYLOAD:",
		"explain ls -la",
		"RESPONSE PAYLOAD:",
		"ls -la lists directory contents in long format.",
	} {
		if !strings.Contains(reqStr, want) {
			t.Errorf("request log file missing %q\nFull content:\n%s", want, reqStr)
		}
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

func TestSessionLoggerMultiTurnIndividualFiles(t *testing.T) {
	tempHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempHome, ".yups"), 0o755)

	logger := New(tempHome, []string{"--", "grep", "foo"})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	// Turn 0: tool call
	req0 := llm.ChatRequest{
		Model: "qwen2.5-coder:7b",
		Messages: []llm.Message{
			{Role: "user", Content: "grep foo"},
		},
	}
	resp0 := llm.ChatResponse{
		Model: "qwen2.5-coder:7b",
		Message: llm.Message{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{
					Function: llm.ToolCallFunction{
						Name:      "fetch-command-documentation",
						Arguments: map[string]any{"command": "grep"},
					},
				},
			},
		},
	}
	logger.LogInteraction(0, "qwen2.5-coder:7b", "http://localhost:11434", req0, resp0, 100*time.Millisecond, nil)

	// Turn 1: final answer
	req1 := llm.ChatRequest{
		Model: "qwen2.5-coder:7b",
		Messages: []llm.Message{
			{Role: "user", Content: "grep foo"},
			{Role: "assistant", Content: "tool response"},
		},
	}
	resp1 := llm.ChatResponse{
		Model: "qwen2.5-coder:7b",
		Message: llm.Message{
			Role:    "assistant",
			Content: `{"explanation":"grep searches text","suggested-command":"grep -rn 'foo' ."}`,
		},
	}
	logger.LogInteraction(1, "qwen2.5-coder:7b", "http://localhost:11434", req1, resp1, 200*time.Millisecond, nil)

	reqDir := logger.RequestsDir()
	entries, err := os.ReadDir(reqDir)
	if err != nil {
		t.Fatalf("reading requestsDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 interaction log files, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Name(), "-turn-0.log") {
		t.Errorf("entry 0 name = %q, want -turn-0.log suffix", entries[0].Name())
	}
	if !strings.Contains(entries[1].Name(), "-turn-1.log") {
		t.Errorf("entry 1 name = %q, want -turn-1.log suffix", entries[1].Name())
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
	if logger.RequestsDir() != "" {
		t.Errorf("expected empty RequestsDir, got %q", logger.RequestsDir())
	}
}

func TestSessionLoggerDoesNotCreateLogsWhenStateDirMissing(t *testing.T) {
	tempHome := t.TempDir()
	// Do NOT create ~/.yups
	logger := New(tempHome, []string{"--help"})
	logger.LogInfo("help displayed")
	logger.LogInteraction(0, "qwen2.5-coder:7b", "http://localhost:11434", llm.ChatRequest{}, llm.ChatResponse{}, 10*time.Millisecond, nil)
	logger.LogIncident("TEST_INCIDENT", "should not create stateDir")
	logger.LogConclusion("", "", "", "HELP", 0)

	stateDir := filepath.Join(tempHome, ".yups")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Errorf("expected stateDir %s to not exist, but it was created", stateDir)
	}
}

func TestSessionLoggerIncidentLogging(t *testing.T) {
	tempHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempHome, ".yups"), 0o755)

	logger := New(tempHome, []string{"--query", "do something bad"})
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}

	logger.LogIncident("TOOL_WHITELIST_REJECTED", "Turn 0: command %q rejected: %s", "cat /etc/shadow", "forbidden token")
	logger.LogIncident("MAX_TURNS_REACHED", "reached maximum turns limit (%d)", 10)
	logger.LogConclusion("", "", "", "ABORTED", 1)

	bufStr := logger.BufferString()
	if !strings.Contains(bufStr, "[! INCIDENT: TOOL_WHITELIST_REJECTED !]") {
		t.Errorf("buffer missing incident header, got:\n%s", bufStr)
	}
	if !strings.Contains(bufStr, "cat /etc/shadow") {
		t.Errorf("buffer missing incident details, got:\n%s", bufStr)
	}

	incidentsPath := logger.IncidentsPath()
	if incidentsPath == "" {
		t.Fatal("expected non-empty IncidentsPath")
	}
	data, err := os.ReadFile(incidentsPath)
	if err != nil {
		t.Fatalf("reading incidents.log: %v", err)
	}
	incidentsStr := string(data)
	if !strings.Contains(incidentsStr, "type=TOOL_WHITELIST_REJECTED") {
		t.Errorf("incidents.log missing TOOL_WHITELIST_REJECTED entry:\n%s", incidentsStr)
	}
	if !strings.Contains(incidentsStr, "type=MAX_TURNS_REACHED") {
		t.Errorf("incidents.log missing MAX_TURNS_REACHED entry:\n%s", incidentsStr)
	}
}

func TestRecordIncidentStandalone(t *testing.T) {
	tempHome := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempHome, ".yups"), 0o755)

	RecordIncident(tempHome, "test-session-123", "yups --install-yups", "INSTALL_ALREADY_INSTALLED", "already installed in /usr/local/bin/yups")

	incidentsPath := filepath.Join(tempHome, ".yups", "logs", "incidents.log")
	data, err := os.ReadFile(incidentsPath)
	if err != nil {
		t.Fatalf("reading incidents.log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "id=test-session-123") ||
		!strings.Contains(content, "type=INSTALL_ALREADY_INSTALLED") ||
		!strings.Contains(content, "cmd=\"yups --install-yups\"") ||
		!strings.Contains(content, "/usr/local/bin/yups") {
		t.Errorf("unexpected incidents.log content:\n%s", content)
	}
}
