package sys

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Colores ANSI
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
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

func (h *YupsHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *YupsHandler) WithGroup(name string) slog.Handler       { return h }
