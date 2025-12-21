package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCommand_Success(t *testing.T) {
	output, err := actualRunner("ls", ".")

	assert.NoError(t, err)
	assert.NotEmpty(t, output, "'ls' output should not be empty")
}

func TestRunCommand_Error(t *testing.T) {
	_, err := actualRunner("ls", "/folder/that/does/not/exist/yups")

	assert.Error(t, err, "Should return an error because the path does not exist")
}

func TestRunSudoCommand_NoTTY_Error(t *testing.T) {
	err := actualSudoRunner("ls")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-interactive terminal",
		"It should detect that there is no tty and the user is not root")
}

func TestIsInteractive(t *testing.T) {
	value := isInteractive()
	assert.False(t, value, "It should detect that there is no tty")
}
