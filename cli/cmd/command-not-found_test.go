package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestHandleCNF(t *testing.T) {
	//TODO
	oldExec := runSudoCommand
	defer func() { runSudoCommand = oldExec }()

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
		})
	}
}
