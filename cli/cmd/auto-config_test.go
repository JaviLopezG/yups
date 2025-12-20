package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	originalRunner := sudoRunner
	sudoRunner = func(name string, args ...string) error {
		fmt.Printf("Mock Sudo: %s %v\n", name, args)
		return nil
	}

	code := m.Run()

	sudoRunner = originalRunner
	os.Exit(code)
}

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
	assert.Equal(t, "linux", viper.GetString("os"))
	assert.NotNil(t, viper.Get("pm"))
	assert.NotNil(t, viper.Get("distro_id"))
	assert.NotNil(t, viper.Get("distro_version"))
	assert.NotNil(t, viper.Get("distro_pretty"))
}
