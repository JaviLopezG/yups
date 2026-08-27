package explain

import (
	"strings"
)

// ControlOperator represents how two command stages are connected.
type ControlOperator string

const (
	OpNone       ControlOperator = ""
	OpOr         ControlOperator = "||"
	OpAnd        ControlOperator = "&&"
	OpPipe       ControlOperator = "|"
	OpPipeStderr ControlOperator = "|&"
	OpSemicolon  ControlOperator = ";"
	OpBackground ControlOperator = "&"
)

// Pipeline represents a full command sequence with control operators.
type Pipeline struct {
	Stages []Stage
}

// Stage represents one command in the pipeline and the operator connecting it
// to the next command.
type Stage struct {
	Command  *Command
	Operator ControlOperator
}

// Flag represents a command line option (short or long).
type Flag struct {
	Raw         string // original raw string e.g. "-al", "--color=auto"
	Name        string // resolved name e.g. "-a", "-l", "--color", "-type"
	Value       string // optional parameter value e.g. "auto"
	IsShort     bool
	IsClustered bool   // whether this flag was part of a clustered short option like -al
	FullWord    string // full word name for single-dash long flags like -type or -name
}

// Redirect represents an I/O redirection.
type Redirect struct {
	Op     string // >, >>, <, 2>, 2>&1, &>
	Target string // target file or descriptor
}

// Wrapper represents a command wrapper (e.g. sudo, time, nohup).
type Wrapper struct {
	Name  string
	Flags []Flag
	Args  []string
}

// Command represents a single parsed command invocation.
type Command struct {
	EnvVars    []string
	Wrappers   []Wrapper
	Name       string
	Subcommand string
	Flags      []Flag
	Args       []string
	Redirects  []Redirect
}

// KnownWrappers lists command wrappers that execute another command.
var KnownWrappers = map[string]bool{
	"sudo":              true,
	"su":                true,
	"runuser":           true,
	"chroot":            true,
	"doas":              true,
	"time":              true,
	"watch":             true,
	"timeout":           true,
	"stdbuf":            true,
	"nohup":             true,
	"xargs":             true,
	"exec":              true,
	"env":               true,
	"strace":            true,
	"nice":              true,
	"runcon":            true,
	"setpriv":           true,
	"bash":              true,
	"sh":                true,
	"pkexec":            true,
	"sg":                true,
	"newgrp":            true,
	"renice":            true,
	"chrt":              true,
	"ionice":            true,
	"taskset":           true,
	"numactl":           true,
	"choom":             true,
	"prlimit":           true,
	"unshare":           true,
	"nsenter":           true,
	"bwrap":             true,
	"aa-exec":           true,
	"capsh":             true,
	"tsp":               true,
	"systemd-run":       true,
	"start-stop-daemon": true,
	"rlwrap":            true,
	"fakeroot":          true,
	"catchsegv":         true,
	"valgrind":          true,
	"setsid":            true,
	"disown":            true,
	"screen":            true,
	"tmux":              true,
	"flock":             true,
}

// KnownSubcommandTools lists tools that typically use subcommands (e.g. git commit, docker run).
var KnownSubcommandTools = map[string]bool{
	"git":        true,
	"docker":     true,
	"podman":     true,
	"kubectl":    true,
	"systemctl":  true,
	"journalctl": true,
	"ip":         true,
	"apt":        true,
	"apt-get":    true,
	"cargo":      true,
	"go":         true,
	"npm":        true,
	"pnpm":       true,
	"yarn":       true,
	"composer":   true,
	"dnf":        true,
	"pacman":     true,
	"zypper":     true,
}

// Parse takes a slice of shell arguments and parses them into a Pipeline.
func Parse(args []string) *Pipeline {
	tokens := Tokenize(args)
	if len(tokens) == 0 {
		return &Pipeline{}
	}

	pipeline := &Pipeline{}
	var currentTokens []Token

	for _, token := range tokens {
		switch token.Type {
		case TokenOpOr, TokenOpAnd, TokenOpPipeStderr, TokenOpPipe, TokenOpSemi, TokenOpBackground:
			if len(currentTokens) > 0 {
				cmd := parseCommand(currentTokens)
				pipeline.Stages = append(pipeline.Stages, Stage{
					Command:  cmd,
					Operator: ControlOperator(token.Value),
				})
				currentTokens = nil
			}
		default:
			currentTokens = append(currentTokens, token)
		}
	}

	if len(currentTokens) > 0 {
		cmd := parseCommand(currentTokens)
		pipeline.Stages = append(pipeline.Stages, Stage{
			Command:  cmd,
			Operator: OpNone,
		})
	}

	return pipeline
}

func parseCommand(tokens []Token) *Command {
	cmd := &Command{}
	i := 0
	n := len(tokens)

	// 1. Parse leading environment variable assignments (e.g. FOO=bar)
	for i < n && tokens[i].Type == TokenWord && isEnvVarAssignment(tokens[i].Value) {
		cmd.EnvVars = append(cmd.EnvVars, tokens[i].Value)
		i++
	}

	// 2. Parse wrappers (e.g. sudo, time, nohup)
	for i < n && tokens[i].Type == TokenWord {
		word := tokens[i].Value
		baseName := filepathBase(word)
		if KnownWrappers[baseName] || KnownWrappers[word] {
			wrapper := Wrapper{Name: word}
			i++
			// Consume flags/args for the wrapper until target command
			for i < n && tokens[i].Type == TokenWord {
				nextWord := tokens[i].Value
				if strings.HasPrefix(nextWord, "-") {
					// Wrapper flag
					wrapper.Flags = append(wrapper.Flags, parseFlags(nextWord)...)
					i++
					// If flag takes argument like -u user
					if (nextWord == "-u" || nextWord == "--user") && i < n && !strings.HasPrefix(tokens[i].Value, "-") {
						wrapper.Args = append(wrapper.Args, tokens[i].Value)
						i++
					}
				} else if isEnvVarAssignment(nextWord) {
					cmd.EnvVars = append(cmd.EnvVars, nextWord)
					i++
				} else {
					break
				}
			}
			cmd.Wrappers = append(cmd.Wrappers, wrapper)
		} else {
			break
		}
	}

	if i >= n {
		return cmd
	}

	// 3. Command name
	if tokens[i].Type == TokenWord {
		cmd.Name = tokens[i].Value
		i++
	}

	// 4. Check for Subcommand if tool supports it (e.g. git commit)
	baseCmd := filepathBase(cmd.Name)
	if KnownSubcommandTools[baseCmd] && i < n && tokens[i].Type == TokenWord && !strings.HasPrefix(tokens[i].Value, "-") && !isEnvVarAssignment(tokens[i].Value) {
		cmd.Subcommand = tokens[i].Value
		i++
	}

	// 5. Parse remaining flags, redirects, and positional arguments
	for i < n {
		tok := tokens[i]
		switch tok.Type {
		case TokenRedir:
			redir := Redirect{Op: tok.Value}
			i++
			if i < n && tokens[i].Type == TokenWord {
				redir.Target = tokens[i].Value
				i++
			}
			cmd.Redirects = append(cmd.Redirects, redir)
		case TokenWord:
			val := tok.Value
			if strings.HasPrefix(val, "-") && val != "-" {
				flags := parseFlags(val)
				cmd.Flags = append(cmd.Flags, flags...)
			} else {
				cmd.Args = append(cmd.Args, val)
			}
			i++
		default:
			i++
		}
	}

	return cmd
}

func isEnvVarAssignment(s string) bool {
	idx := strings.Index(s, "=")
	if idx <= 0 {
		return false
	}
	name := s[:idx]
	for i, r := range name {
		if i == 0 && (r >= '0' && r <= '9') {
			return false
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func parseFlags(raw string) []Flag {
	if raw == "-" || raw == "--" {
		return nil
	}

	// Case 1: Long option (--flag or --flag=value)
	if strings.HasPrefix(raw, "--") {
		content := raw[2:]
		if idx := strings.Index(content, "="); idx != -1 {
			name := "--" + content[:idx]
			val := content[idx+1:]
			return []Flag{{
				Raw:     raw,
				Name:    name,
				Value:   val,
				IsShort: false,
			}}
		}
		return []Flag{{
			Raw:     raw,
			Name:    raw,
			IsShort: false,
		}}
	}

	// Case 2: Single-dash option (-a, -al, -type)
	if strings.HasPrefix(raw, "-") {
		content := raw[1:]
		if len(content) == 1 {
			// Single short flag e.g. -a
			return []Flag{{
				Raw:     raw,
				Name:    raw,
				IsShort: true,
			}}
		}

		// Multiple chars: could be clustered (-al) OR single-dash long option (-type, -name)
		var flags []Flag
		for _, r := range content {
			flags = append(flags, Flag{
				Raw:         raw,
				Name:        "-" + string(r),
				IsShort:     true,
				IsClustered: true,
				FullWord:    raw,
			})
		}
		return flags
	}

	return nil
}

func filepathBase(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}
