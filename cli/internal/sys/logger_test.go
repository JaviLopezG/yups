// internal/sys/logger_test.go
package sys

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYupsHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewYupsHandler(&buf, slog.LevelInfo)
	logger := slog.New(handler)

	// Test de nivel info
	logger.Info("test message", "key", "val")

	output := buf.String()
	assert.Contains(t, output, "[INFO]")
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=val")
}

func TestYupsHandler_Enabled(t *testing.T) {
	handler := NewYupsHandler(nil, slog.LevelWarn)

	// No debería permitir Info, pero sí Error
	assert.False(t, handler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, handler.Enabled(context.Background(), slog.LevelError))
}
