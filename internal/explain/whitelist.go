package explain

import (
	"fmt"
	"strings"
)

// WhitelistedCommands defines the set of safe, read-only inspection commands
// that the LLM is permitted to invoke via the 'command-run' tool.
var WhitelistedCommands = map[string]bool{
	"ls":         true,
	"pwd":        true,
	"stat":       true,
	"file":       true,
	"du":         true,
	"df":         true,
	"find":       true,
	"locate":     true,
	"tree":       true,
	"cat":        true,
	"less":       true,
	"more":       true,
	"grep":       true,
	"egrep":      true,
	"fgrep":      true,
	"head":       true,
	"tail":       true,
	"diff":       true,
	"cmp":        true,
	"wc":         true,
	"md5sum":     true,
	"sha256sum":  true,
	"ps":         true,
	"top":        true,
	"htop":       true,
	"free":       true,
	"uptime":     true,
	"lscpu":      true,
	"lspci":      true,
	"lsusb":      true,
	"lshw":       true,
	"dmidecode":  true,
	"ip":         true,
	"ss":         true,
	"ping":       true,
	"traceroute": true,
	"mtr":        true,
	"dig":        true,
	"nslookup":   true,
	"host":       true,
	"uname":      true,
	"which":      true,
	"whereis":    true,
}

// ValidateWhitelistedCommand parses a shell command line (which may contain
// Bash operators like &&, ||, |, ;, &) and verifies that every individual command
// invoked belongs to the allowed whitelist.
func ValidateWhitelistedCommand(cmdLine string) (bool, string) {
	trimmed := strings.TrimSpace(cmdLine)
	if trimmed == "" {
		return false, "empty command line"
	}

	pipeline := Parse([]string{trimmed})
	if len(pipeline.Stages) == 0 {
		return false, "failed to parse command pipeline"
	}

	for _, stage := range pipeline.Stages {
		if stage.Command == nil || stage.Command.Name == "" {
			return false, "empty command stage detected"
		}

		cmdName := stage.Command.Name
		if !WhitelistedCommands[cmdName] {
			return false, fmt.Sprintf("command %q is not in the whitelist", cmdName)
		}

		for _, w := range stage.Command.Wrappers {
			if !WhitelistedCommands[w.Name] {
				return false, fmt.Sprintf("wrapper %q is not in the whitelist", w.Name)
			}
		}
	}

	return true, ""
}
