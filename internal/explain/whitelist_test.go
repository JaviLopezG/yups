package explain

import (
	"testing"
)

func TestValidateWhitelistedCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdLine string
		allowed bool
	}{
		{
			name:    "simple allowed command",
			cmdLine: "ls -la",
			allowed: true,
		},
		{
			name:    "compound pipeline of allowed commands",
			cmdLine: "ps aux | grep yups && uptime",
			allowed: true,
		},
		{
			name:    "disallowed binary in pipeline",
			cmdLine: "ls -l | rm -rf /tmp",
			allowed: false,
		},
		{
			name:    "disallowed standalone binary",
			cmdLine: "curl https://evil.com",
			allowed: false,
		},
		{
			name:    "wrapper not in whitelist",
			cmdLine: "sudo ls -la",
			allowed: false,
		},
		{
			name:    "complex valid bash combinator",
			cmdLine: "df -h ; free -m && lscpu | head -n 10",
			allowed: true,
		},
		{
			name:    "empty command line",
			cmdLine: "   ",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ValidateWhitelistedCommand(tt.cmdLine)
			if got != tt.allowed {
				t.Errorf("ValidateWhitelistedCommand(%q) = %v (reason: %q), want %v", tt.cmdLine, got, reason, tt.allowed)
			}
		})
	}
}
