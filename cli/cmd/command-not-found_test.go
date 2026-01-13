package cmd

import (
	"strings"
	"testing"

	"github.com/javilopezg/yups/cli/internal/sys"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestHandleCNF(t *testing.T) {
	//TODO mock sys.Runner
	tests := []struct {
		name          string
		fullCmd       string
		lastCmd       string
		args          []string
		mockOutput    string
		expectedInLog string
	}{
		{
			name:          "Simple command",
			fullCmd:       "nano",
			lastCmd:       "nano",
			args:          []string{"nano"},
			mockOutput:    "nano-8.5-2.fc43.x86_64",
			expectedInLog: "nano-8.5-2.fc43.x86_64",
		},
		{ //FIXME this case is not real, sudo doesn't exit with 127
			name:          "Command with sudo",
			fullCmd:       "sudo nano",
			lastCmd:       "sudo nano",
			args:          []string{"sudo", "nano"},
			mockOutput:    "nano-8.5-2.fc43.x86_64",
			expectedInLog: "nano-8.5-2.fc43.x86_64",
		},
		{
			name:          "Complex chain with &&",
			fullCmd:       "nano && echo 'ok'",
			lastCmd:       "nano",
			args:          []string{"nano", "&&", "echo", "ok"},
			mockOutput:    "nano-8.5-2.fc43.x86_64",
			expectedInLog: "nano-8.5-2.fc43.x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("pm", "dnf")
			viper.Set("YUPS_LAST_CMD", tt.lastCmd)

			handleCNF(tt.args)
			//TODO Asserts

			assert.Equal(t, "", "")
		})
	}
}

func TestHandleCNF_Mocked(t *testing.T) {
	oldRunner := sys.Runner
	defer func() { sys.Runner = oldRunner }()

	sys.Runner = func(provides string, args ...string) (string, error) {
		if strings.Contains(provides, "provides") {
			return "nano-8.5-2.fc43.x86_64", nil
		}
		return "", nil
	}

	viper.Set("pm", "dnf")
	viper.Set("YUPS_LAST_CMD", "nano")

	handleCNF([]string{"nano"})

	//TODO Agrega aserciones aquí
}
