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
			name:    "compgen and type inspection commands",
			cmdLine: "compgen -c | grep -i helo ; type helo",
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
			name:    "allowed wrapper with allowed command",
			cmdLine: "sudo ls -la",
			allowed: true,
		},
		{
			name:    "allowed wrapper with disallowed command",
			cmdLine: "sudo rm -rf /tmp/foo",
			allowed: false,
		},
		{
			name:    "disallowed wrapper",
			cmdLine: "unshare --mount ls -la",
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
		// rsync dry-run tests
		{
			name:    "rsync with -n",
			cmdLine: "rsync -avz -n /src/ /dst/",
			allowed: true,
		},
		{
			name:    "rsync with --dry-run",
			cmdLine: "rsync --dry-run /src/ /dst/",
			allowed: true,
		},
		{
			name:    "rsync without dry-run",
			cmdLine: "rsync -avz /src/ /dst/",
			allowed: false,
		},
		// make dry-run tests
		{
			name:    "make with -n",
			cmdLine: "make -n all",
			allowed: true,
		},
		{
			name:    "make with -p",
			cmdLine: "make -p",
			allowed: true,
		},
		{
			name:    "make without flags",
			cmdLine: "make build",
			allowed: false,
		},
		// bash/sh syntax check tests
		{
			name:    "bash with -n",
			cmdLine: "bash -n script.sh",
			allowed: true,
		},
		{
			name:    "bash without -n",
			cmdLine: "bash script.sh",
			allowed: false,
		},
		// apt / apt-get tests
		{
			name:    "apt search",
			cmdLine: "apt search nginx",
			allowed: true,
		},
		{
			name:    "apt list",
			cmdLine: "apt list --installed",
			allowed: true,
		},
		{
			name:    "apt install with -s",
			cmdLine: "apt install -s nginx",
			allowed: true,
		},
		{
			name:    "apt install without simulation",
			cmdLine: "apt install nginx",
			allowed: false,
		},
		// dnf / yum tests
		{
			name:    "dnf search",
			cmdLine: "dnf search python3",
			allowed: true,
		},
		{
			name:    "dnf install with test tsflags",
			cmdLine: "dnf install --setopt=tsflags=test python3",
			allowed: true,
		},
		{
			name:    "dnf install without test flags",
			cmdLine: "dnf install python3",
			allowed: false,
		},
		// pacman tests
		{
			name:    "pacman query",
			cmdLine: "pacman -Q",
			allowed: true,
		},
		{
			name:    "pacman search",
			cmdLine: "pacman -Ss gcc",
			allowed: true,
		},
		{
			name:    "pacman install with -p",
			cmdLine: "pacman -Sp gcc",
			allowed: true,
		},
		{
			name:    "pacman install without print",
			cmdLine: "pacman -S gcc",
			allowed: false,
		},
		// pip tests
		{
			name:    "pip list",
			cmdLine: "pip list",
			allowed: true,
		},
		{
			name:    "pip install with --dry-run",
			cmdLine: "pip install --dry-run flask",
			allowed: true,
		},
		{
			name:    "pip install without dry-run",
			cmdLine: "pip install flask",
			allowed: false,
		},
		// git tests
		{
			name:    "git status",
			cmdLine: "git status",
			allowed: true,
		},
		{
			name:    "git log",
			cmdLine: "git log --oneline -n 5",
			allowed: true,
		},
		{
			name:    "git diff",
			cmdLine: "git diff HEAD~1",
			allowed: true,
		},
		{
			name:    "git clean with -n",
			cmdLine: "git clean -n -d",
			allowed: true,
		},
		{
			name:    "git clean without dry-run",
			cmdLine: "git clean -fd",
			allowed: false,
		},
		{
			name:    "git push with --dry-run",
			cmdLine: "git push --dry-run origin main",
			allowed: true,
		},
		{
			name:    "git push without dry-run",
			cmdLine: "git push origin main",
			allowed: false,
		},
		// cargo tests
		{
			name:    "cargo check",
			cmdLine: "cargo check",
			allowed: true,
		},
		{
			name:    "cargo publish with --dry-run",
			cmdLine: "cargo publish --dry-run",
			allowed: true,
		},
		{
			name:    "cargo build without dry-run",
			cmdLine: "cargo build --release",
			allowed: false,
		},
		// npm tests
		{
			name:    "npm list",
			cmdLine: "npm list -g",
			allowed: true,
		},
		{
			name:    "npm install with --dry-run",
			cmdLine: "npm i --dry-run express",
			allowed: true,
		},
		{
			name:    "npm install without dry-run",
			cmdLine: "npm install express",
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
