package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/tu-usuario/yups/cli/internal/sys"
)

func TestCheckConfig_UserResponse(t *testing.T) {
	oldPrompt := sys.PromptConfirmReplacement
	oldConfigFile := viper.ConfigFileUsed()
	defer func() {
		sys.PromptConfirmReplacement = oldPrompt
		viper.SetConfigFile(oldConfigFile)
	}()

	sys.PromptConfirmReplacement = func(command string) (bool, error) {
		return false, nil
	}

	viper.SetConfigFile("/non/existent/path")
	err := checkConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "needs to be configured")
}
