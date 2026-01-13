func TestHandleAC(t *testing.T) {
	// 1. Reset de Viper para limpiar rastro de otros tests
	viper.Reset()

	// 2. Aislamiento del Entorno
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// 3. Crear el entorno mínimo
	fakeBashrc := filepath.Join(tmpDir, ".bashrc")
	os.WriteFile(fakeBashrc, []byte("# Fake bashrc\n"), 0644)

	// 4. Configurar Viper para el test ANTES de handleAC
	configPath := filepath.Join(tmpDir, ".yups", "config.toml")
	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml") // Importante para que sepa cómo serializar

	// Mock del servidor de descarga (mismo código que tenías)
	mockContent := "fake-model-content"
	hash := sha256.Sum256([]byte(mockContent))
	expectedHash := hex.EncodeToString(hash[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockContent))
	}))
	defer server.Close()

	// Swapping de globales
	oldUri, oldHash, oldPath := modelUri, modelHash, yupsPath
	modelUri, modelHash = server.URL, expectedHash
	yupsPath = filepath.Join(tmpDir, "fake_yups_bin")
	defer func() { modelUri, modelHash, yupsPath = oldUri, oldHash, oldPath }()

	// 5. Ejecutar
	handleAC()

	// 6. Aserciones
	assert.FileExists(t, configPath, "El archivo de configuración debería existir")
	
	// Recargamos para verificar contenido
	viper.ReadInConfig()
	assert.Equal(t, "linux", viper.GetString("os"))
	assert.Equal(t, "info", viper.GetString("log_level"))
}
