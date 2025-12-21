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
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			value, _ := ExtractEffectiveCommand(tt.command)
			assert.Equal(t, tt.want, value)
		})
	}
}
