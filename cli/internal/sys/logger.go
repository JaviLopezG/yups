package sys

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Colores ANSI
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

type YupsHandler struct {
	fileWriter io.Writer
	mu         sync.Mutex
	level      slog.Level
}

func NewYupsHandler(file io.Writer, level slog.Level) *YupsHandler {
	return &YupsHandler{
		fileWriter: file,
		level:      level,
	}
}

func (h *YupsHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *YupsHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrs := ""
	r.Attrs(func(a slog.Attr) bool {
		attrs += fmt.Sprintf(" %s=%v", a.Key, a.Value)
		return true // continuar iterando
	})

	if h.fileWriter != nil {
		fmt.Fprintf(h.fileWriter, "[%s] [%s] %s%s\n",
			r.Time.Format(time.RFC3339), r.Level, r.Message, attrs)
	}

	var out io.Writer = os.Stdout
	var color string
	var levelLabel string

	switch r.Level {
	case slog.LevelDebug:
		color = colorGreen
		levelLabel = "DEBUG: "
	case slog.LevelInfo:
		color = colorWhite
		levelLabel = ""
	case slog.LevelWarn:
		color = colorYellow
		levelLabel = "WARN: "
	case slog.LevelError:
		color = colorRed
		levelLabel = "ERROR: "
		out = os.Stderr
	}

	fmt.Fprintf(out, "%s%s%s%s%s%s\n", color, levelLabel, r.Message, colorReset, colorWhite, attrs)
	return nil
}

func Prompt(message string) {
	fmt.Printf("%s%s %s[Y/n]:%s ", colorWhite, message, colorYellow, colorReset)
}

func Step(step int, steps int, message string) {
	fmt.Printf("%s[%d/%d]%s %s...\n", colorCyan, step, step, colorReset, message)
}

type ProgressWriter struct {
	Total, Current uint64
	Message        string
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Current += uint64(n)
	percentage := float64(pw.Current) / float64(pw.Total) * 100
	fmt.Printf("\r\t📥 %s: [%-50s] %.1f%%", pw.Message,
		strings.Repeat("=", int(percentage/2)), percentage)
	return n, nil
}

func (h *YupsHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *YupsHandler) WithGroup(name string) slog.Handler       { return h }
