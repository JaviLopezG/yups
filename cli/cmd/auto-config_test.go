package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestHandleAC(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	viper.SetConfigFile(configPath)

	handleAC()

	_, err := os.Stat(configPath)
	if os.IsNotExist(err) {
		t.Fatalf("File not found at %s", configPath)
	}

	err = viper.ReadInConfig()
	assert.NoError(t, err)
	assert.Equal(t, "info", viper.GetString("log_level"))
}
