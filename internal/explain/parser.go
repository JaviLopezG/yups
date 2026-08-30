package explain

import (
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"
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
	Stages  []Stage
	Comment string
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
	Comment    string
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
	"yum":        true,
	"pip":        true,
	"pip3":       true,
	"pacman":     true,
	"zypper":     true,
}

// Parse takes a slice of shell arguments and parses them into a Pipeline using mvdan.cc/sh/v3/syntax.
func Parse(args []string) *Pipeline {
	if len(args) == 0 {
		return &Pipeline{}
	}

	raw := joinArgs(args)
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &Pipeline{}
	}

	if isNaturalLanguageQuery(trimmed) && !strings.HasPrefix(trimmed, "#") {
		return &Pipeline{
			Comment: trimmed,
		}
	}

	p := syntax.NewParser(syntax.KeepComments(true), syntax.Variant(syntax.LangBash))
	file, err := p.Parse(strings.NewReader(raw), "")
	if err != nil {
		// If bash parsing fails, treat the input as a natural language or comment query
		return &Pipeline{
			Comment: trimmed,
		}
	}

	pipeline := &Pipeline{}

	// Extract top-level comments
	var commentParts []string
	syntax.Walk(file, func(node syntax.Node) bool {
		if c, ok := node.(*syntax.Comment); ok {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "#"))
			if text != "" {
				commentParts = append(commentParts, text)
			}
		}
		return true
	})
	if len(commentParts) > 0 {
		pipeline.Comment = strings.Join(commentParts, " ")
	}

	// Flatten statements into pipeline stages
	for i, stmt := range file.Stmts {
		trailingOp := OpNone
		if stmt.Background {
			trailingOp = OpBackground
		} else if i < len(file.Stmts)-1 {
			trailingOp = OpSemicolon
		}
		stages := flattenStmt(stmt, trailingOp)
		pipeline.Stages = append(pipeline.Stages, stages...)
	}

	// If there are no command stages but a comment was found
	if len(pipeline.Stages) == 0 && pipeline.Comment != "" {
		return pipeline
	}

	// Propagate comment to command if present
	if pipeline.Comment != "" && len(pipeline.Stages) > 0 {
		lastCmd := pipeline.Stages[len(pipeline.Stages)-1].Command
		if lastCmd != nil && lastCmd.Comment == "" {
			lastCmd.Comment = pipeline.Comment
		}
	}

	return pipeline
}

func flattenStmt(stmt *syntax.Stmt, trailingOp ControlOperator) []Stage {
	if stmt == nil || stmt.Cmd == nil {
		return nil
	}

	switch x := stmt.Cmd.(type) {
	case *syntax.BinaryCmd:
		op := mapBinOp(x.Op)
		left := flattenStmt(x.X, op)
		right := flattenStmt(x.Y, trailingOp)
		return append(left, right...)
	case *syntax.CallExpr:
		cmd := parseCallExpr(x, stmt.Redirs, stmt.Comments)
		return []Stage{{Command: cmd, Operator: trailingOp}}
	case *syntax.Subshell:
		var stages []Stage
		for i, s := range x.Stmts {
			op := OpNone
			if s.Background {
				op = OpBackground
			} else if i < len(x.Stmts)-1 {
				op = OpSemicolon
			}
			if i == len(x.Stmts)-1 && trailingOp != OpNone {
				op = trailingOp
			}
			stages = append(stages, flattenStmt(s, op)...)
		}
		return stages
	case *syntax.Block:
		var stages []Stage
		for i, s := range x.Stmts {
			op := OpNone
			if s.Background {
				op = OpBackground
			} else if i < len(x.Stmts)-1 {
				op = OpSemicolon
			}
			if i == len(x.Stmts)-1 && trailingOp != OpNone {
				op = trailingOp
			}
			stages = append(stages, flattenStmt(s, op)...)
		}
		return stages
	case *syntax.TimeClause:
		if x.Stmt != nil {
			stages := flattenStmt(x.Stmt, trailingOp)
			if len(stages) > 0 && stages[0].Command != nil {
				stages[0].Command.Wrappers = append([]Wrapper{{Name: "time"}}, stages[0].Command.Wrappers...)
			}
			return stages
		}
		return []Stage{{Command: &Command{Name: "time"}, Operator: trailingOp}}
	default:
		// Fallback for other shell clauses
		var sb strings.Builder
		_ = syntax.NewPrinter().Print(&sb, stmt)
		cmd := &Command{
			Name: strings.TrimSpace(sb.String()),
		}
		return []Stage{{Command: cmd, Operator: trailingOp}}
	}
}

func mapBinOp(op syntax.BinCmdOperator) ControlOperator {
	switch op {
	case syntax.AndStmt:
		return OpAnd
	case syntax.OrStmt:
		return OpOr
	case syntax.Pipe:
		return OpPipe
	case syntax.PipeAll:
		return OpPipeStderr
	default:
		return OpNone
	}
}

func parseCallExpr(call *syntax.CallExpr, redirs []*syntax.Redirect, comments []syntax.Comment) *Command {
	cmd := &Command{}

	// 1. EnvVars from Assigns (e.g. FOO=bar)
	for _, a := range call.Assigns {
		if a.Name != nil {
			val := ""
			if a.Value != nil {
				val = wordValue(a.Value)
			}
			cmd.EnvVars = append(cmd.EnvVars, a.Name.Value+"="+val)
		}
	}

	// 2. Extract word strings from args
	var words []string
	for _, w := range call.Args {
		words = append(words, wordValue(w))
	}

	i := 0
	n := len(words)

	// 3. Leading environment variable assignments in arguments
	for i < n && isEnvVarAssignment(words[i]) {
		cmd.EnvVars = append(cmd.EnvVars, words[i])
		i++
	}

	// 4. Wrappers (e.g. sudo, time, nohup)
	for i < n {
		w := words[i]
		base := filepathBase(w)
		if KnownWrappers[base] || KnownWrappers[w] {
			wrapper := Wrapper{Name: w}
			i++
			for i < n {
				next := words[i]
				if strings.HasPrefix(next, "-") {
					wrapper.Flags = append(wrapper.Flags, parseFlags(next)...)
					i++
					if (next == "-u" || next == "--user") && i < n && !strings.HasPrefix(words[i], "-") {
						wrapper.Args = append(wrapper.Args, words[i])
						i++
					}
				} else if isEnvVarAssignment(next) {
					cmd.EnvVars = append(cmd.EnvVars, next)
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

	// 5. Command name
	cmd.Name = words[i]
	i++

	// 6. Subcommand check for tools like git, apt, cargo, docker
	baseCmd := filepathBase(cmd.Name)
	if KnownSubcommandTools[baseCmd] && i < n && !strings.HasPrefix(words[i], "-") && !isEnvVarAssignment(words[i]) {
		cmd.Subcommand = words[i]
		i++
	}

	// 7. Flags and positional arguments
	for i < n {
		val := words[i]
		if strings.HasPrefix(val, "-") && val != "-" {
			cmd.Flags = append(cmd.Flags, parseFlags(val)...)
		} else {
			cmd.Args = append(cmd.Args, val)
		}
		i++
	}

	// 8. Redirections
	for _, r := range redirs {
		redir := Redirect{Op: r.Op.String()}
		if r.N != nil {
			redir.Op = r.N.Value + redir.Op
		}
		if r.Word != nil {
			redir.Target = wordValue(r.Word)
		}
		if r.Op == syntax.DplOut && r.N != nil && r.N.Value == "2" && r.Word != nil && wordValue(r.Word) == "1" {
			redir.Op = "2>&1"
			redir.Target = ""
		}
		cmd.Redirects = append(cmd.Redirects, redir)
	}

	// 9. Comments
	for _, c := range comments {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "#"))
		if text != "" {
			if cmd.Comment != "" {
				cmd.Comment += " " + text
			} else {
				cmd.Comment = text
			}
		}
	}

	return cmd
}

func wordValue(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range w.Parts {
		writeWordPart(&sb, part)
	}
	return sb.String()
}

func writeWordPart(sb *strings.Builder, part syntax.WordPart) {
	switch p := part.(type) {
	case *syntax.Lit:
		sb.WriteString(p.Value)
	case *syntax.SglQuoted:
		sb.WriteString(p.Value)
	case *syntax.DblQuoted:
		for _, inner := range p.Parts {
			writeWordPart(sb, inner)
		}
	case *syntax.ParamExp:
		if p.Param != nil {
			if p.Short {
				sb.WriteString("$" + p.Param.Value)
			} else {
				sb.WriteString("${" + p.Param.Value + "}")
			}
		}
	case *syntax.CmdSubst:
		var sub strings.Builder
		_ = syntax.NewPrinter().Print(&sub, p)
		sb.WriteString(sub.String())
	case *syntax.ArithmExp:
		var sub strings.Builder
		_ = syntax.NewPrinter().Print(&sub, p)
		sb.WriteString(sub.String())
	case *syntax.ProcSubst:
		var sub strings.Builder
		_ = syntax.NewPrinter().Print(&sub, p)
		sb.WriteString(sub.String())
	default:
		var sub strings.Builder
		_ = syntax.NewPrinter().Print(&sub, p)
		sb.WriteString(sub.String())
	}
}

func isNaturalLanguageQuery(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "¿") || strings.HasPrefix(trimmed, "?") {
		return true
	}
	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"cómo ", "como ", "how ", "how to ", "what ", "what is ", "where ",
		"why ", "quién ", "quien ", "cual ", "cuál ", "dónde ", "donde ",
		"ayuda ", "help ", "explicar ", "explain ", "muéstrame ", "show me ",
		"dime ", "tell me ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func joinArgs(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	var sb strings.Builder
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if i > 0 {
			sb.WriteByte(' ')
		}
		if strings.HasPrefix(arg, "#") {
			sb.WriteString(strings.Join(args[i:], " "))
			break
		}
		if needsQuotes(arg) {
			sb.WriteString(quoteArg(arg))
		} else {
			sb.WriteString(arg)
		}
	}
	return sb.String()
}

func needsQuotes(s string) bool {
	if s == "" {
		return true
	}
	switch s {
	case "||", "&&", "|", "|&", ";", "&", ">", ">>", "<", "2>", "2>&1", "&>":
		return false
	}
	if strings.ContainsAny(s, "|&;><#") {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '\'' || r == '\\' || r == '$' || r == '`' {
			return true
		}
	}
	return false
}

func quoteArg(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' || r == '$' || r == '`' {
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	sb.WriteByte('"')
	return sb.String()
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
