package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractEffectiveCommand(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{
			command: "echo \"hello world\"",
			want:    "echo",
		},
		{
			command: "nano -flag -b /folder/file",
			want:    "nano",
		},
		{
			command: "sudo nano file",
			want:    "nano",
		},
		{
			command: "/bin/nano file",
			want:    "nano",
		},
		{
			command: "sudo /bin/nano file",
			want:    "nano",
		},
		{
			command: "env -i /bin/nano file",
			want:    "nano",
		},
		{
			command: "env foo=bar nano",
			want:    "nano",
		},
		{
			command: "ls -l /nonexistent | grep foo",
			want:    "ls",
		},
		{
			command: "mkdir new_dir && cd new_dir",
			want:    "mkdir",
		},
		{
			command: "(cd /tmp && ls)",
			want:    "cd",
		},
		{
			command: "../../local/bin/my-script --run",
			want:    "my-script",
		},
		{
			command: "sudo",
			want:    "",
		},
		{
			command: "DEBUG=1 ./run.sh",
			want:    "run.sh",
		},
		{
			command: "cat file.txt > output.log",
			want:    "cat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			value, _ := ExtractEffectiveCommand(tt.command)
			assert.Equal(t, tt.want, value)
		})
	}
}

func TestExtractEffectiveCommand_Errors(t *testing.T) {
	// Sintaxis inválida (comilla abierta sin cerrar)
	command := "echo \"hello"
	value, err := ExtractEffectiveCommand(command)

	assert.Error(t, err, "Should return an error for invalid shell syntax")
	assert.Equal(t, "", value)
}
