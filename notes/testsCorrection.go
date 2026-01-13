package cmd

import (
	"testing"
	"github.com/JaviLopezG/yups/cli/internal/sys"
)

func TestSomething(t *testing.T) {
	// Guardamos el original para restaurarlo después
	old := sys.PromptConfirmReplacement
	defer func() { sys.PromptConfirmReplacement = old }()

	// Mocking: Asignamos una nueva función a la variable del paquete sys
	sys.PromptConfirmReplacement = func(pkg string) bool {
		return true
	}

	// ... resto del test
}
