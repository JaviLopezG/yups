// Package sessionlog implements execution trace and Ollama interaction logging
// stored in ~/.yups/logs/.
package sessionlog

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"yups/assets"
	"yups/internal/llm"
)

// GenerateSessionSlug generates a random human-readable slug (e.g. "sevilla37", "barcelona23").
func GenerateSessionSlug() string {
	cities := assets.GetSessionCities()
	city := "yups"
	if len(cities) > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(cities))))
		if err == nil {
			city = cities[n.Int64()]
		} else {
			city = cities[0]
		}
	}
	digitBig, err := rand.Int(rand.Reader, big.NewInt(100))
	digit := 0
	if err == nil {
		digit = int(digitBig.Int64())
	}
	return fmt.Sprintf("%s%02d", city, digit)
}

// SessionLogger captures step-by-step decisions and Ollama interactions for a yups run.
type SessionLogger struct {
	mu            sync.Mutex
	homeDir       string
	sessionID     string
	slug          string
	logPath       string
	summaryPath   string
	incidentsPath string
	requestsDir   string
	startTime     time.Time
	pid           int
	commandLine   string
	args          []string
	buf           bytes.Buffer
	turnCount     int
	modelUsed     string
	written       bool
	closed        bool
	disabled      bool
}

// New initializes a new SessionLogger for the given user home directory and CLI arguments.
func New(homeDir string, args []string) *SessionLogger {
	if homeDir == "" {
		return &SessionLogger{disabled: true}
	}

	now := time.Now()
	pid := os.Getpid()
	slug := GenerateSessionSlug()
	sessionID := fmt.Sprintf("%s-%d-%s", now.Format("20060102-150405"), pid, slug)
	logsDir := filepath.Join(homeDir, ".yups", "logs")
	sessionDirName := fmt.Sprintf("session-%s", sessionID)

	cmdStr := "yups"
	if len(args) > 0 {
		cmdStr += " " + strings.Join(args, " ")
	}

	l := &SessionLogger{
		homeDir:       homeDir,
		sessionID:     sessionID,
		slug:          slug,
		logPath:       filepath.Join(logsDir, fmt.Sprintf("%s.log", sessionDirName)),
		summaryPath:   filepath.Join(logsDir, "sessions.log"),
		incidentsPath: filepath.Join(logsDir, "incidents.log"),
		requestsDir:   filepath.Join(logsDir, "llm-requests", sessionDirName),
		startTime:     now,
		pid:           pid,
		commandLine:   cmdStr,
		args:          args,
		disabled:      false,
	}

	l.buf.WriteString("================================================================================\n")
	l.buf.WriteString(fmt.Sprintf("YUPS SESSION LOG: %s\n", sessionID))
	l.buf.WriteString("================================================================================\n")
	l.buf.WriteString(fmt.Sprintf("Session ID:  %s\n", sessionID))
	l.buf.WriteString(fmt.Sprintf("Slug:        %s\n", slug))
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
func (l *SessionLogger) LogConfig(endpoint, defModel, advModel string, llmEnabled bool, llmTimeout, toolTimeout time.Duration, maxTurns, maxBytes, advMult int, noLimits bool) {
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
	l.buf.WriteString(fmt.Sprintf("No Limits (--no-limits):  %t\n", noLimits))
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

// LogInteraction records an entire exchange with Ollama /api/chat both in the
// in-memory session trace and in a dedicated per-interaction file under llm-requests/.
func (l *SessionLogger) LogInteraction(turn int, model, endpoint string, req llm.ChatRequest, resp llm.ChatResponse, duration time.Duration, err error) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.turnCount++
	l.modelUsed = model
	now := time.Now()

	l.buf.WriteString(fmt.Sprintf("\n>>> [OLLAMA INTERACTION Turn %d] >>>\n", turn))
	l.buf.WriteString(fmt.Sprintf("Timestamp: %s | Duration: %v | Endpoint: %s | Model: %s\n",
		now.Format("15:04:05.000"), duration, endpoint, model))

	reqJSON, jsonErr := json.MarshalIndent(req, "", "  ")
	if jsonErr == nil {
		l.buf.WriteString("Request Payload:\n")
		l.buf.WriteString(string(reqJSON))
		l.buf.WriteString("\n")
	}

	if err != nil {
		l.buf.WriteString(fmt.Sprintf("<<< ERROR: %v <<<\n", err))
	} else {
		respJSON, jErr := json.MarshalIndent(resp, "", "  ")
		if jErr == nil {
			l.buf.WriteString("<<< Response Payload:\n")
			l.buf.WriteString(string(respJSON))
			l.buf.WriteString("\n<<< END INTERACTION <<<\n")
		}
	}

	// Write dedicated per-interaction request/response file under ~/.yups/logs/llm-requests/session-.../
	if !l.disabled && l.homeDir != "" {
		stateDir := filepath.Join(l.homeDir, ".yups")
		if _, statErr := os.Stat(stateDir); statErr == nil {
			if mkErr := os.MkdirAll(l.requestsDir, 0o755); mkErr == nil {
				reqFileName := fmt.Sprintf("request-%s-turn-%d.log", now.Format("20060102-150405"), turn)
				reqFilePath := filepath.Join(l.requestsDir, reqFileName)

				var reqBuf bytes.Buffer
				reqBuf.WriteString("================================================================================\n")
				reqBuf.WriteString("YUPS LLM REQUEST / RESPONSE INTERACTION\n")
				reqBuf.WriteString("================================================================================\n")
				reqBuf.WriteString(fmt.Sprintf("Session ID:  %s\n", l.sessionID))
				reqBuf.WriteString(fmt.Sprintf("Turn:        %d\n", turn))
				reqBuf.WriteString(fmt.Sprintf("Timestamp:   %s\n", now.Format(time.RFC3339)))
				reqBuf.WriteString(fmt.Sprintf("Model:       %s\n", model))
				reqBuf.WriteString(fmt.Sprintf("Endpoint:    %s\n", endpoint))
				reqBuf.WriteString(fmt.Sprintf("Duration:    %v\n", duration))
				if err != nil {
					reqBuf.WriteString(fmt.Sprintf("Error:       %v\n", err))
				} else {
					reqBuf.WriteString("Error:       none\n")
				}
				reqBuf.WriteString("--------------------------------------------------------------------------------\n")
				reqBuf.WriteString("REQUEST PAYLOAD:\n")
				reqBuf.WriteString("--------------------------------------------------------------------------------\n")
				if jsonErr == nil {
					reqBuf.Write(reqJSON)
				}
				reqBuf.WriteString("\n\n--------------------------------------------------------------------------------\n")
				reqBuf.WriteString("RESPONSE PAYLOAD:\n")
				reqBuf.WriteString("--------------------------------------------------------------------------------\n")
				if err != nil {
					reqBuf.WriteString(fmt.Sprintf("ERROR: %v\n", err))
				} else {
					respJSON, jErr := json.MarshalIndent(resp, "", "  ")
					if jErr == nil {
						reqBuf.Write(respJSON)
					}
					reqBuf.WriteString("\n")
				}
				reqBuf.WriteString("================================================================================\n")

				_ = os.WriteFile(reqFilePath, reqBuf.Bytes(), 0o644)
			}
		}
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

// LogIncident records an anomalous event, rejected action, or controlled exception
// both in the session trace and in the aggregated ~/.yups/logs/incidents.log file.
func (l *SessionLogger) LogIncident(category string, format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	details := fmt.Sprintf(format, args...)
	now := time.Now()

	l.buf.WriteString(fmt.Sprintf("\n[! INCIDENT: %s !]\n", category))
	l.buf.WriteString(fmt.Sprintf("Details: %s\n", details))

	if l.disabled || l.homeDir == "" {
		return
	}

	stateDir := filepath.Join(l.homeDir, ".yups")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return
	}

	logsDir := filepath.Dir(l.incidentsPath)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return
	}

	incidentLine := fmt.Sprintf("[%s] id=%s type=%s cmd=%q details=%q\n",
		now.Format(time.RFC3339),
		l.sessionID,
		category,
		l.commandLine,
		details,
	)

	if f, err := os.OpenFile(l.incidentsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_, _ = f.WriteString(incidentLine)
		_ = f.Close()
	}
}

// RecordIncident records an incident directly into ~/.yups/logs/incidents.log
// when a SessionLogger instance is not in scope.
func RecordIncident(homeDir, sessionID, cmdStr, category string, format string, args ...any) {
	if homeDir == "" {
		return
	}
	stateDir := filepath.Join(homeDir, ".yups")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return
	}

	logsDir := filepath.Join(homeDir, ".yups", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return
	}

	incidentsPath := filepath.Join(logsDir, "incidents.log")
	now := time.Now()
	details := fmt.Sprintf(format, args...)
	if sessionID == "" {
		sessionID = fmt.Sprintf("%s-%d", now.Format("20060102-150405"), os.Getpid())
	}
	if cmdStr == "" {
		cmdStr = "yups"
	}

	incidentLine := fmt.Sprintf("[%s] id=%s type=%s cmd=%q details=%q\n",
		now.Format(time.RFC3339),
		sessionID,
		category,
		cmdStr,
		details,
	)

	if f, err := os.OpenFile(incidentsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_, _ = f.WriteString(incidentLine)
		_ = f.Close()
	}
}

// LogLimitReached records when turn limit or timeout was reached and user response.
func (l *SessionLogger) LogLimitReached(limitType, details string, userAborted bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.buf.WriteString(fmt.Sprintf("\n[! LIMIT REACHED: %s !]\n", limitType))
	l.buf.WriteString(fmt.Sprintf("Details:      %s\n", details))
	l.buf.WriteString(fmt.Sprintf("User Aborted: %t\n", userAborted))
	l.mu.Unlock()

	l.LogIncident("LIMIT_REACHED", "type=%s details=%s userAborted=%t", limitType, details, userAborted)
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
	l.written = true

	// Append one-line summary to sessions.log
	modelStr := l.modelUsed
	if modelStr == "" {
		modelStr = "local-doc"
	}
	summaryLine := fmt.Sprintf("[%s] id=%s slug=%s pid=%d cmd=%q model=%q turns=%d status=%s duration=%s file=%s\n",
		l.startTime.Format(time.RFC3339),
		l.sessionID,
		l.slug,
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

// SessionID returns the unique session ID string.
func (l *SessionLogger) SessionID() string {
	if l == nil {
		return ""
	}
	return l.sessionID
}

// Slug returns the human-readable city slug (e.g. "sevilla37").
func (l *SessionLogger) Slug() string {
	if l == nil {
		return ""
	}
	return l.slug
}

// HasWritten returns true if a session log was committed to disk.
func (l *SessionLogger) HasWritten() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.written
}

// IsDisabled returns true if session logging is disabled.
func (l *SessionLogger) IsDisabled() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.disabled
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

// IncidentsPath returns the path to the aggregated incidents log file.
func (l *SessionLogger) IncidentsPath() string {
	if l == nil {
		return ""
	}
	return l.incidentsPath
}

// RequestsDir returns the path to the directory containing per-interaction LLM logs.
func (l *SessionLogger) RequestsDir() string {
	if l == nil {
		return ""
	}
	return l.requestsDir
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

// LastSession stores summarized data and conversational state from the most recent run.
type LastSession struct {
	SessionID        string
	Slug             string
	CommandLine      string
	ModelUsed        string
	Explanation      string
	SuggestedCommand string
	SuggestedScript  string
	Conversation     []llm.Message
}

// LoadLastSession loads the most recent recorded session from ~/.yups/logs/.
func LoadLastSession(homeDir string) (*LastSession, error) {
	if homeDir == "" {
		return nil, fmt.Errorf("home directory is empty")
	}
	logsDir := filepath.Join(homeDir, ".yups", "logs")
	summaryPath := filepath.Join(logsDir, "sessions.log")

	data, err := os.ReadFile(summaryPath)
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("no sessions found in %s: %w", summaryPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("sessions.log is empty")
	}

	var lastLine string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			lastLine = trimmed
			break
		}
	}
	if lastLine == "" {
		return nil, fmt.Errorf("no valid session entry found in sessions.log")
	}

	sess := &LastSession{}
	if idx := strings.Index(lastLine, "id="); idx != -1 {
		rest := lastLine[idx+3:]
		if end := strings.Index(rest, " "); end != -1 {
			sess.SessionID = rest[:end]
		}
	}
	if idx := strings.Index(lastLine, "slug="); idx != -1 {
		rest := lastLine[idx+5:]
		if end := strings.Index(rest, " "); end != -1 {
			sess.Slug = rest[:end]
		}
	}
	if idx := strings.Index(lastLine, "cmd="); idx != -1 {
		rest := lastLine[idx+4:]
		if len(rest) > 0 && rest[0] == '"' {
			if end := strings.Index(rest[1:], `"`); end != -1 {
				sess.CommandLine = rest[1 : end+1]
			}
		}
	}
	if idx := strings.Index(lastLine, "model="); idx != -1 {
		rest := lastLine[idx+6:]
		if len(rest) > 0 && rest[0] == '"' {
			if end := strings.Index(rest[1:], `"`); end != -1 {
				sess.ModelUsed = rest[1 : end+1]
			}
		}
	}

	sessionLogFile := filepath.Join(logsDir, fmt.Sprintf("session-%s.log", sess.SessionID))
	if logData, err := os.ReadFile(sessionLogFile); err == nil {
		logStr := string(logData)
		if explIdx := strings.Index(logStr, "Final Explanation: "); explIdx != -1 {
			rest := logStr[explIdx+len("Final Explanation: "):]
			if end := strings.Index(rest, "\n"); end != -1 {
				sess.Explanation = strings.TrimSpace(rest[:end])
			}
		}
		if cmdIdx := strings.Index(logStr, "Suggested Command: "); cmdIdx != -1 {
			rest := logStr[cmdIdx+len("Suggested Command: "):]
			if end := strings.Index(rest, "\n"); end != -1 {
				sess.SuggestedCommand = strings.TrimSpace(rest[:end])
			}
		}
		if scriptIdx := strings.Index(logStr, "Suggested Script:  "); scriptIdx != -1 {
			rest := logStr[scriptIdx+len("Suggested Script:  "):]
			if end := strings.Index(rest, "================================================================================"); end != -1 {
				sess.SuggestedScript = strings.TrimSpace(rest[:end])
			} else {
				sess.SuggestedScript = strings.TrimSpace(rest)
			}
		}
	}

	requestsDir := filepath.Join(logsDir, "llm-requests", fmt.Sprintf("session-%s", sess.SessionID))
	if reqEntries, err := os.ReadDir(requestsDir); err == nil && len(reqEntries) > 0 {
		var lastReqFile string
		for _, re := range reqEntries {
			if !re.IsDir() && strings.HasPrefix(re.Name(), "request-") && strings.HasSuffix(re.Name(), ".log") {
				lastReqFile = filepath.Join(requestsDir, re.Name())
			}
		}
		if lastReqFile != "" {
			if rfData, err := os.ReadFile(lastReqFile); err == nil {
				rfStr := string(rfData)
				if reqStart := strings.Index(rfStr, "REQUEST PAYLOAD:\n--------------------------------------------------------------------------------\n"); reqStart != -1 {
					rest := rfStr[reqStart+len("REQUEST PAYLOAD:\n--------------------------------------------------------------------------------\n"):]
					if reqEnd := strings.Index(rest, "\n\n--------------------------------------------------------------------------------\nRESPONSE PAYLOAD:"); reqEnd != -1 {
						reqJSON := rest[:reqEnd]
						var chatReq llm.ChatRequest
						if err := json.Unmarshal([]byte(reqJSON), &chatReq); err == nil {
							sess.Conversation = append(sess.Conversation, chatReq.Messages...)
						}
					}
				}
				if respStart := strings.Index(rfStr, "RESPONSE PAYLOAD:\n--------------------------------------------------------------------------------\n"); respStart != -1 {
					rest := rfStr[respStart+len("RESPONSE PAYLOAD:\n--------------------------------------------------------------------------------\n"):]
					if respEnd := strings.Index(rest, "\n================================================================================"); respEnd != -1 {
						respJSON := rest[:respEnd]
						var chatResp llm.ChatResponse
						if err := json.Unmarshal([]byte(respJSON), &chatResp); err == nil && chatResp.Message.Content != "" {
							sess.Conversation = append(sess.Conversation, chatResp.Message)
						}
					}
				}
			}
		}
	}

	return sess, nil
}
