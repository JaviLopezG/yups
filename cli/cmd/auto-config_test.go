package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/javilopezg/yups/cli/internal/sys"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	originalRunner := sys.SudoRunner
	sys.SudoRunner = func(name string, args ...string) error {
		fmt.Printf("[Test Mock] Sudo: %s %v\n", name, args)
		return nil
	}

	code := m.Run()

	sys.SudoRunner = originalRunner
	os.Exit(code)
}

func TestHandleAC(t *testing.T) {
	// 1. Aislamiento del Entorno
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Crear un .bashrc falso para que updateBashrc tenga algo que leer
	fakeBashrc := filepath.Join(tmpDir, ".bashrc")
	os.WriteFile(fakeBashrc, []byte("# Fake bashrc\n"), 0644)

	// 2. Mock del Servidor de Descarga para el Modelo
	mockContent := "fake-model-content\n" + string(make([]byte, math.MaxInt32))
	hash := sha256.Sum256([]byte(mockContent))
	expectedHash := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mockContent)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockContent))
	}))
	defer server.Close()

	// 3. Sobrescribir variables globales para el test
	originalUri := modelUri
	originalHash := modelHash
	originalPath := yupsPath
	modelUri = server.URL
	modelHash = expectedHash
	yupsPath = filepath.Join(tmpDir, "fake_yups_bin")

	defer func() {
		modelUri = originalUri
		modelHash = originalHash
		yupsPath = originalPath
	}()

	// 4. Ejecución del Test
	configPath := filepath.Join(tmpDir, ".yups", "config.toml")
	viper.Reset()
	viper.SetConfigFile(configPath)

	handleAC()

	// 5. Aserciones
	// Verificar Configuración
	assert.FileExists(t, configPath)
	viper.ReadInConfig()
	assert.Equal(t, "linux", viper.GetString("os"))

	// Verificar Bashrc (debe contener los hooks)
	bashContent, _ := os.ReadFile(fakeBashrc)
	assert.Contains(t, string(bashContent), hookStart)

	// Verificar Modelo (debe existir y ser el archivo correcto)
	modelPath := filepath.Join(tmpDir, ".yups", "models", "gemma-3-270m.gguf")
	assert.FileExists(t, modelPath)

	downloadedContent, _ := os.ReadFile(modelPath)
	assert.Equal(t, mockContent, string(downloadedContent))
}

func TestHandleAR(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)
	viper.Reset()

	// Simular instalación previa
	yupsDir := filepath.Join(tmpDir, ".yups")
	os.MkdirAll(yupsDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, ".bashrc"), []byte(hookStart+"\n"+hookEnd), 0644)

	handleAR()

	// Verificar que se eliminó la carpeta de configuración
	_, err := os.Stat(yupsDir)
	assert.True(t, os.IsNotExist(err), "La carpeta .yups debería haber sido eliminada")

	// Verificar que el bashrc ya no tiene los hooks
	bashContent, _ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))
	assert.NotContains(t, string(bashContent), hookStart)
}
