func saveConfigFile(info sys.Info) {
	viper.Set("os", info.OS)
	viper.Set("pm", info.PM)
	viper.Set("distro_id", info.DistroID)
	viper.Set("distro_version", info.DistroVersion)
	viper.Set("distro_pretty", info.DistroPretty)
	viper.Set("log_level", "info")

	configPath := viper.ConfigFileUsed()
	// Si por alguna razón está vacío (no debería), forzamos la ruta por defecto
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".yups", "config.toml")
	}

	// Aseguramos que el directorio existe
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		slog.Error("Could not create config directory", "error", err)
		return
	}

	// Usamos WriteConfigAs para evitar que Viper intente "adivinar" o validar 
	// configuraciones faltantes de búsqueda (evita el error de 'configPath')
	if err := viper.WriteConfigAs(configPath); err != nil {
		slog.Error("Error writing config file", "file", configPath, "error", err)
	} else {
		slog.Debug("Config file saved successfully", "path", configPath)
	}
}
