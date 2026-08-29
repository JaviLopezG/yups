// Package sessionlog implements execution trace and Ollama interaction logging
// stored in ~/.yups/logs/.
package sessionlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yups/internal/llm"
)

// SessionLogger captures step-by-step decisions and Ollama interactions for a yups run.
type SessionLogger struct {
	mu          sync.Mutex
	homeDir     string
	sessionID   string
	logPath     string
	summaryPath string
	startTime   time.Time
	pid         int
	commandLine string
	args        []string
	buf         bytes.Buffer
	turnCount   int
	modelUsed   string
	closed      bool
	disabled    bool
}

// New initializes a new SessionLogger for the given user home directory and CLI arguments.
func New(homeDir string, args []string) *SessionLogger {
	if homeDir == "" {
		return &SessionLogger{disabled: true}
	}

	now := time.Now()
	pid := os.Getpid()
	sessionID := fmt.Sprintf("%s-%d", now.Format("20060102-150405"), pid)
	logsDir := filepath.Join(homeDir, ".yups", "logs")

	cmdStr := "yups"
	if len(args) > 0 {
		cmdStr += " " + strings.Join(args, " ")
	}

	l := &SessionLogger{
		homeDir:     homeDir,
		sessionID:   sessionID,
		logPath:     filepath.Join(logsDir, fmt.Sprintf("session-%s.log", sessionID)),
		summaryPath: filepath.Join(logsDir, "sessions.log"),
		startTime:   now,
		pid:         pid,
		commandLine: cmdStr,
		args:        args,
		disabled:    false,
	}

	l.buf.WriteString("================================================================================\n")
	l.buf.WriteString(fmt.Sprintf("YUPS SESSION LOG: %s\n", sessionID))
	l.buf.WriteString("================================================================================\n")
	l.buf.WriteString(fmt.Sprintf("Timestamp:   %s\n", now.Format(time.RFC3339)))
	l.buf.WriteString(fmt.Sprintf("PID:         %d\n", pid))
	l.buf.WriteString(fmt.Sprintf("Command:     %s\n", cmdStr))
	l.buf.WriteString(fmt.Sprintf("Raw Args:    %s\n", formatArgs(args)))
	if cwd, err := os.Getwd(); err == nil {
		l.buf.WriteString(fmt.Sprintf("Working Dir: %s\n", cwd))
	}
	l.buf.WriteString("--------------------------------------------------------------------------------\n\n")

	return l
}

func formatArgs(args []string) string {
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(b)
}

// LogSection writes a section header to the session log.
func (l *SessionLogger) LogSection(title string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.WriteString(fmt.Sprintf("\n--- [%s] ---\n", title))
}

// LogInfo writes an informational message to the session log.
func (l *SessionLogger) LogInfo(format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	l.buf.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05.000"), msg))
}

// LogConfig records loaded configuration settings.
func (l *SessionLogger) LogConfig(endpoint, defModel, advModel string, llmEnabled bool, llmTimeout, toolTimeout time.Duration, maxTurns, maxBytes, advMult int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf.WriteString("\n--- [CONFIGURATION & LIMITS] ---\n")
	l.buf.WriteString(fmt.Sprintf("Inference Endpoint:       %s\n", endpoint))
	l.buf.WriteString(fmt.Sprintf("Default Model:            %s\n", defModel))
	l.buf.WriteString(fmt.Sprintf("Advanced Model:           %s\n", advModel))
	l.buf.WriteString(fmt.Sprintf("LLM Enabled:              %t\n", llmEnabled))
	l.buf.WriteString(fmt.Sprintf("LLM Timeout:              %v\n", llmTimeout))
	l.buf.WriteString(fmt.Sprintf("Tool Execution Timeout:   %v\n", toolTimeout))
	l.buf.WriteString(fmt.Sprintf("Max Tool Turns:           %d\n", maxTurns))
	l.buf.WriteString(fmt.Sprintf("Max Tool Output Bytes:    %d\n", maxBytes))
	l.buf.WriteString(fmt.Sprintf("Advanced Multiplier:      %d\n", advMult))
}

// LogModelResolution records why a particular model was chosen.
func (l *SessionLogger) LogModelResolution(model string, isAdvanced bool, reason string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.modelUsed = model
	l.buf.WriteString("\n--- [TARGET MODEL SELECTION] ---\n")
	l.buf.WriteString(fmt.Sprintf("Selected Model: %s (advanced=%t)\n", model, isAdvanced))
	l.buf.WriteString(fmt.Sprintf("Rationale:      %s\n", reason))
}

// LogInteraction records an entire exchange with Ollama /api/chat.
func (l *SessionLogger) LogInteraction(turn int, model, endpoint string, req llm.ChatRequest, resp llm.ChatResponse, duration time.Duration, err error) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.turnCount++
	l.modelUsed = model

	l.buf.WriteString(fmt.Sprintf("\n>>> [OLLAMA INTERACTION Turn %d] >>>\n", turn))
	l.buf.WriteString(fmt.Sprintf("Timestamp: %s | Duration: %v | Endpoint: %s | Model: %s\n",
		time.Now().Format("15:04:05.000"), duration, endpoint, model))

	reqJSON, jsonErr := json.MarshalIndent(req, "", "  ")
	if jsonErr == nil {
		l.buf.WriteString("Request Payload:\n")
		l.buf.WriteString(string(reqJSON))
		l.buf.WriteString("\n")
	}

	if err != nil {
		l.buf.WriteString(fmt.Sprintf("<<< ERROR: %v <<<\n", err))
		return
	}

	respJSON, jsonErr := json.MarshalIndent(resp, "", "  ")
	if jsonErr == nil {
		l.buf.WriteString("<<< Response Payload:\n")
		l.buf.WriteString(string(respJSON))
		l.buf.WriteString("\n<<< END INTERACTION <<<\n")
	}
}

// LogToolExecution records a tool call execution and output.
func (l *SessionLogger) LogToolExecution(turn int, toolName string, args map[string]any, whitelisted bool, reason, output string, duration time.Duration, err error) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf.WriteString(fmt.Sprintf("\n--- [TOOL EXECUTION Turn %d: %s] ---\n", turn, toolName))
	argsJSON, _ := json.Marshal(args)
	l.buf.WriteString(fmt.Sprintf("Arguments:      %s\n", string(argsJSON)))
	l.buf.WriteString(fmt.Sprintf("Whitelisted:    %t (reason: %s)\n", whitelisted, reason))
	l.buf.WriteString(fmt.Sprintf("Duration:       %v\n", duration))
	if err != nil {
		l.buf.WriteString(fmt.Sprintf("Execution Err:  %v\n", err))
	}
	l.buf.WriteString("Output:\n")
	l.buf.WriteString(output)
	if !strings.HasSuffix(output, "\n") {
		l.buf.WriteString("\n")
	}
	l.buf.WriteString("--- [END TOOL EXECUTION] ---\n")
}

// LogLimitReached records when turn limit or timeout was reached and user response.
func (l *SessionLogger) LogLimitReached(limitType, details string, userAborted bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.buf.WriteString(fmt.Sprintf("\n[! LIMIT REACHED: %s !]\n", limitType))
	l.buf.WriteString(fmt.Sprintf("Details:      %s\n", details))
	l.buf.WriteString(fmt.Sprintf("User Aborted: %t\n", userAborted))
}

// LogConclusion finalizes the session log and writes summary to sessions.log.
func (l *SessionLogger) LogConclusion(explanation, suggestedCmd, suggestedScript, status string, exitCode int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}
	l.closed = true

	duration := time.Since(l.startTime)
	l.buf.WriteString("\n================================================================================\n")
	l.buf.WriteString("SESSION CONCLUSION\n")
	l.buf.WriteString("================================================================================\n")
	l.buf.WriteString(fmt.Sprintf("Total Duration:    %v\n", duration))
	l.buf.WriteString(fmt.Sprintf("Total Turns:       %d\n", l.turnCount))
	l.buf.WriteString(fmt.Sprintf("Exit Code:         %d\n", exitCode))
	l.buf.WriteString(fmt.Sprintf("Status:            %s\n", status))
	if explanation != "" {
		l.buf.WriteString(fmt.Sprintf("Final Explanation: %s\n", explanation))
	}
	if suggestedCmd != "" {
		l.buf.WriteString(fmt.Sprintf("Suggested Command: %s\n", suggestedCmd))
	}
	if suggestedScript != "" {
		l.buf.WriteString(fmt.Sprintf("Suggested Script:  %s\n", suggestedScript))
	}
	l.buf.WriteString("================================================================================\n")

	if l.disabled {
		return
	}

	// Only write logs if ~/.yups state directory exists (do not create it for uninstalled runs)
	stateDir := filepath.Join(l.homeDir, ".yups")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return
	}

	// Create logs directory on demand before writing
	logsDir := filepath.Dir(l.logPath)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return
	}

	// Write detailed session log file
	_ = os.WriteFile(l.logPath, l.buf.Bytes(), 0o644)

	// Append one-line summary to sessions.log
	modelStr := l.modelUsed
	if modelStr == "" {
		modelStr = "local-doc"
	}
	summaryLine := fmt.Sprintf("[%s] id=%s pid=%d cmd=%q model=%q turns=%d status=%s duration=%s file=%s\n",
		l.startTime.Format(time.RFC3339),
		l.sessionID,
		l.pid,
		l.commandLine,
		modelStr,
		l.turnCount,
		status,
		duration.Round(time.Millisecond),
		filepath.Base(l.logPath),
	)

	if f, err := os.OpenFile(l.summaryPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_, _ = f.WriteString(summaryLine)
		_ = f.Close()
	}
}

// Disable permanently disables writing session logs to disk.
func (l *SessionLogger) Disable() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.disabled = true
	l.closed = true
}

// LogPath returns the path to the detailed session log file.
func (l *SessionLogger) LogPath() string {
	if l == nil {
		return ""
	}
	return l.logPath
}

// BufferString returns the in-memory buffered log content (useful for testing).
func (l *SessionLogger) BufferString() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}
