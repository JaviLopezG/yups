package explain

import (
	"fmt"
	"strings"
)

// WhitelistedCommands defines the set of unconditionally safe, read-only inspection commands
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
	"compgen":    true,
	"type":       true,
	"alias":      true,
	"echo":       true,
	"cd":         true,
}

// WhitelistedWrappers defines safe wrappers that can prefix an inspection command.
var WhitelistedWrappers = map[string]bool{
	"sudo":  true,
	"env":   true,
	"time":  true,
	"nohup": true,
	"nice":  true,
}

// ConditionalCommandValidators maps commands that are allowed only when invoked
// with read-only subcommands or dry-run/simulation flags.
var ConditionalCommandValidators = map[string]func(cmd *Command) (bool, string){
	"rsync":   validateRsync,
	"make":    validateMake,
	"bash":    validateBashOrSh,
	"sh":      validateBashOrSh,
	"apt":     validateApt,
	"apt-get": validateApt,
	"dnf":     validateDnfOrYum,
	"yum":     validateDnfOrYum,
	"pacman":  validatePacman,
	"pip":     validatePip,
	"pip3":    validatePip,
	"git":     validateGit,
	"cargo":   validateCargo,
	"npm":     validateNpmOrYarn,
	"pnpm":    validateNpmOrYarn,
	"yarn":    validateNpmOrYarn,
}

// hasFlag checks whether the command contains any of the target flag names.
func hasFlag(cmd *Command, names ...string) bool {
	if cmd == nil {
		return false
	}
	for _, f := range cmd.Flags {
		for _, name := range names {
			if f.Name == name || f.Raw == name || f.FullWord == name {
				return true
			}
			if strings.HasPrefix(f.Raw, name+"=") {
				return true
			}
		}
	}
	for _, arg := range cmd.Args {
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return true
			}
		}
	}
	return false
}

func validateRsync(cmd *Command) (bool, string) {
	if hasFlag(cmd, "-n", "--dry-run") {
		return true, ""
	}
	return false, "command 'rsync' requires dry-run flag (-n or --dry-run)"
}

func validateMake(cmd *Command) (bool, string) {
	if hasFlag(cmd, "-n", "--dry-run", "--recon", "--just-print", "-q", "--question", "-p", "--print-data-base") {
		return true, ""
	}
	return false, "command 'make' requires dry-run or inspection flag (-n, --dry-run, -p, -q)"
}

func validateBashOrSh(cmd *Command) (bool, string) {
	if hasFlag(cmd, "-n") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires -n (syntax check only without execution)", cmd.Name)
}

func validateApt(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"search": true, "show": true, "list": true, "policy": true,
		"depends": true, "rdepends": true, "showpkg": true, "showsrc": true,
		"changelog": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if hasFlag(cmd, "-s", "--simulate", "--dry-run", "--just-print", "--recon", "--no-act") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires a read-only subcommand (e.g. search, show, list, policy) or a simulation flag (-s, --dry-run)", cmd.Name)
}

func validateDnfOrYum(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"search": true, "info": true, "list": true, "repoinfo": true,
		"repolist": true, "check-update": true, "provides": true,
		"whatprovides": true, "deplist": true, "repoquery": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if hasFlag(cmd, "--setopt=tsflags=test", "--assumeno", "--downloadonly") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires a read-only subcommand (e.g. search, info, list, provides) or a dry-run flag (--setopt=tsflags=test, --assumeno, --downloadonly)", cmd.Name)
}

func validateWrapper(w Wrapper) (bool, string) {
	if WhitelistedWrappers[w.Name] || WhitelistedCommands[w.Name] {
		return true, ""
	}
	if w.Name == "bash" || w.Name == "sh" {
		for _, f := range w.Flags {
			if f.Name == "-n" || f.Raw == "-n" || f.FullWord == "-n" {
				return true, ""
			}
		}
		for _, arg := range w.Args {
			if arg == "-n" {
				return true, ""
			}
		}
		return false, fmt.Sprintf("wrapper %q requires -n (syntax check only without execution)", w.Name)
	}
	return false, fmt.Sprintf("wrapper %q is not in the whitelist", w.Name)
}

func validatePacman(cmd *Command) (bool, string) {
	for _, f := range cmd.Flags {
		if strings.HasPrefix(f.Name, "-Q") || strings.HasPrefix(f.Raw, "-Q") || strings.HasPrefix(f.FullWord, "-Q") ||
			f.Raw == "-Ss" || f.Raw == "-Si" || f.Raw == "-Sl" || f.Raw == "-Sg" ||
			strings.HasPrefix(f.Raw, "-F") || strings.HasPrefix(f.Name, "-F") {
			return true, ""
		}
	}
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "-Q") || arg == "-Ss" || arg == "-Si" || arg == "-Sl" || strings.HasPrefix(arg, "-F") {
			return true, ""
		}
	}
	if hasFlag(cmd, "-p", "--print", "--print-format") {
		return true, ""
	}
	return false, "command 'pacman' requires a query operation (-Q, -Ss, -Si, -F) or a dry-run flag (-p, --print)"
}

func validatePip(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"list": true, "show": true, "search": true, "check": true, "inspect": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if hasFlag(cmd, "--dry-run") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires a read-only subcommand (list, show, check, inspect) or --dry-run", cmd.Name)
}

func validateGit(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"status": true, "log": true, "diff": true, "show": true,
		"rev-parse": true, "describe": true, "shortlog": true, "blame": true,
		"ls-files": true, "ls-remote": true, "ls-tree": true, "cat-file": true,
		"check-ref-format": true, "verify-pack": true, "grep": true,
		"count-objects": true, "for-each-ref": true, "name-rev": true,
		"show-ref": true, "var": true, "version": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if sub == "branch" || sub == "tag" || sub == "remote" || sub == "config" || sub == "stash" {
		if !hasFlag(cmd, "-d", "-D", "-m", "-M", "-a", "--delete", "--move", "add", "rm", "remove", "set-url", "rename", "drop", "pop", "clear") {
			return true, ""
		}
	}
	if hasFlag(cmd, "-n", "--dry-run", "--check") {
		return true, ""
	}
	return false, fmt.Sprintf("command 'git' with subcommand %q requires a dry-run flag (-n, --dry-run, --check)", cmd.Subcommand)
}

func validateCargo(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"check": true, "search": true, "read-manifest": true, "tree": true,
		"verify-project": true, "metadata": true, "report": true, "locate-project": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if hasFlag(cmd, "--dry-run") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires a read-only subcommand (check, search, tree, metadata) or --dry-run", cmd.Name)
}

func validateNpmOrYarn(cmd *Command) (bool, string) {
	readOnlySubs := map[string]bool{
		"list": true, "ls": true, "view": true, "info": true, "show": true,
		"search": true, "outdated": true, "why": true, "audit": true,
		"explain": true, "docs": true, "repo": true, "bugs": true, "fund": true,
	}
	sub := strings.ToLower(cmd.Subcommand)
	if readOnlySubs[sub] {
		return true, ""
	}
	if hasFlag(cmd, "--dry-run", "-n") {
		return true, ""
	}
	return false, fmt.Sprintf("command %q requires a read-only subcommand (e.g. list, view, outdated, audit) or --dry-run", cmd.Name)
}

// ValidateWhitelistedCommand parses a shell command line (which may contain
// Bash operators like &&, ||, |, ;, &) and verifies that every individual command
// invoked belongs to the allowed whitelist or meets the safe dry-run conditions.
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

		cmd := stage.Command
		cmdName := cmd.Name

		isSyntaxCheckOnly := false
		for _, w := range cmd.Wrappers {
			allowed, reason := validateWrapper(w)
			if !allowed {
				return false, reason
			}
			if (w.Name == "bash" || w.Name == "sh") && hasFlagOnWrapper(w, "-n") {
				isSyntaxCheckOnly = true
			}
		}

		if isSyntaxCheckOnly {
			// In `bash -n script.sh`, the target script is not executed, only syntax-checked
			continue
		}

		if WhitelistedCommands[cmdName] {
			continue
		}

		if validator, ok := ConditionalCommandValidators[cmdName]; ok {
			allowed, reason := validator(cmd)
			if !allowed {
				return false, reason
			}
			continue
		}

		return false, fmt.Sprintf("command %q is not in the whitelist", cmdName)
	}

	return true, ""
}

func hasFlagOnWrapper(w Wrapper, flagName string) bool {
	for _, f := range w.Flags {
		if f.Name == flagName || f.Raw == flagName || f.FullWord == flagName {
			return true
		}
	}
	for _, a := range w.Args {
		if a == flagName {
			return true
		}
	}
	return false
}
