package sys

import (
	//"io/fs"
	"os"
	"path/filepath"
	//"strings"
)

// ListAllCommands devuelve un slice con todos los binarios ejecutables en el PATH
func ListAllCommands() ([]string, error) {
	pathEnv := os.Getenv("PATH")
	// SplitList maneja automáticamente los separadores (: en Linux, ; en Windows)
	dirs := filepath.SplitList(pathEnv)

	// Usamos un mapa para evitar duplicados (set)
	commandsMap := make(map[string]bool)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Si una carpeta del PATH no se puede leer, la ignoramos y seguimos
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Verificamos si es un archivo regular y si tiene permisos de ejecución
			// 0111 es la máscara octal para --x--x--x (ejecutable para alguien)
			if !entry.IsDir() && (info.Mode()&0111 != 0) {
				commandsMap[entry.Name()] = true
			}
		}
	}

	// Convertir mapa a slice
	var commands []string
	for cmd := range commandsMap {
		commands = append(commands, cmd)
	}

	return commands, nil
}
