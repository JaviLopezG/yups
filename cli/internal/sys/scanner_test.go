// internal/sys/scanner_test.go
package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListAllCommands(t *testing.T) {
	cmds, err := ListAllCommands()
	assert.NoError(t, err)
	assert.NotEmpty(t, cmds)

	// Comprobamos si encuentra comandos básicos
	found := false
	for _, c := range cmds {
		if c == "ls" || c == "bash" || c == "sh" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find basic system commands in PATH")
}
