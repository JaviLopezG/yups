package explain

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"yups/internal/llm"
)

// DocEnv defines the OS interface methods needed for documentation retrieval.
type DocEnv struct {
	RunCmdTimeout func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error)
	Whatis        func(ctx context.Context, cmd string) (string, error)
	ManPage       func(ctx context.Context, cmd string) (string, error)
	TypeCmd       func(ctx context.Context, cmd string) (string, error)
	StatPath      func(path string) (fs.FileInfo, error)
	LookupInPath  func(cmd string) bool

	// LLM support
	LLMClient     *llm.Client
	LLMEnv        llm.LLMEnv
	DefaultModel  string
	AdvancedModel string
}

// Resolver coordinates documentation lookup across multiple sources.
type Resolver struct {
	env       DocEnv
	helpCache map[string]string // "cmd [subcmd]" -> help text
	manCache  map[string]string // "cmd" or "cmd-subcmd" -> man content
}

// NewResolver creates a documentation Resolver backed by the given DocEnv.
func NewResolver(env DocEnv) *Resolver {
	return &Resolver{
		env:       env,
		helpCache: make(map[string]string),
		manCache:  make(map[string]string),
	}
}

// ExplainPipeline resolves documentation for an entire parsed Pipeline.
func (r *Resolver) ExplainPipeline(ctx context.Context, pipeline *Pipeline) *PipelineExplanation {
	if pipeline == nil || len(pipeline.Stages) == 0 {
		return &PipelineExplanation{}
	}

	result := &PipelineExplanation{}
	for _, stage := range pipeline.Stages {
		stageExp := StageExplanation{
			Operator:  stage.Operator,
			OpSummary: describeOperator(stage.Operator),
		}
		if stage.Command != nil {
			stageExp.Command = r.ExplainCommand(ctx, stage.Command)
		}
		result.Stages = append(result.Stages, stageExp)
	}

	return result
}

// ExplainCommand resolves documentation for a single command.
func (r *Resolver) ExplainCommand(ctx context.Context, cmd *Command) *CommandExplanation {
	if cmd == nil || cmd.Name == "" {
		return &CommandExplanation{}
	}

	exp := &CommandExplanation{
		Name:       cmd.Name,
		Subcommand: cmd.Subcommand,
		EnvVars:    cmd.EnvVars,
	}

	// 1. Check wrappers (e.g. sudo, time)
	for _, w := range cmd.Wrappers {
		wExp := WrapperExplanation{
			Name: w.Name,
			Args: w.Args,
		}
		if r.env.Whatis != nil {
			if s, err := r.env.Whatis(ctx, w.Name); err == nil && s != "" {
				wExp.Summary = firstNonEmptyLine(s)
			}
		}
		for _, flag := range w.Flags {
			wExp.Flags = append(wExp.Flags, r.lookupFlagDoc(ctx, w.Name, "", flag))
		}
		exp.Wrappers = append(exp.Wrappers, wExp)
	}

	// 2. Type / Alias / Builtin check
	if r.env.TypeCmd != nil {
		if out, err := r.env.TypeCmd(ctx, cmd.Name); err == nil && out != "" {
			firstLine := firstNonEmptyLine(out)
			if strings.Contains(firstLine, "alias") {
				exp.AliasInfo = firstLine
			} else if strings.Contains(firstLine, "builtin") || strings.Contains(firstLine, "interna") {
				exp.BuiltinInfo = firstLine
			}
		}
	}

	// 3. Command summary (whatis -> man NAME -> help header)
	exp.Summary = r.lookupSummary(ctx, cmd.Name, cmd.Subcommand)
	exp.Found = exp.Summary != "" || exp.AliasInfo != "" || exp.BuiltinInfo != "" || (r.env.LookupInPath != nil && r.env.LookupInPath(cmd.Name))

	// 4. Flags documentation (Help first -> fallback to Man)
	// Track already explained full-word flags (e.g. -type vs -t -y -p -e)
	handledFullWords := make(map[string]bool)
	for _, flag := range cmd.Flags {
		if flag.FullWord != "" && handledFullWords[flag.FullWord] {
			continue
		}

		// If this is part of a single-dash multi-character cluster (e.g. -type or -al):
		// Check if the full word exists as a dedicated flag first.
		if flag.FullWord != "" && flag.IsClustered {
			fullFlag := Flag{
				Raw:     flag.FullWord,
				Name:    flag.FullWord,
				IsShort: true,
			}
			fullDoc := r.lookupFlagDoc(ctx, cmd.Name, cmd.Subcommand, fullFlag)
			if fullDoc.Found {
				exp.Flags = append(exp.Flags, fullDoc)
				handledFullWords[flag.FullWord] = true
				continue
			}
		}

		flagDoc := r.lookupFlagDoc(ctx, cmd.Name, cmd.Subcommand, flag)
		exp.Flags = append(exp.Flags, flagDoc)
	}

	// 5. Positional arguments inspection
	for _, arg := range cmd.Args {
		argExp := ArgExplanation{Value: arg}
		if r.env.StatPath != nil {
			if fi, err := r.env.StatPath(arg); err == nil {
				if fi.IsDir() {
					argExp.Kind = "directory"
				} else {
					argExp.Kind = "file"
				}
			} else {
				argExp.Kind = "argument"
			}
		} else {
			argExp.Kind = "argument"
		}
		exp.PositionalArgs = append(exp.PositionalArgs, argExp)
	}

	// 6. Redirects
	for _, redir := range cmd.Redirects {
		exp.Redirects = append(exp.Redirects, RedirectExplanation{
			Op:          redir.Op,
			Target:      redir.Target,
			Description: describeRedirect(redir.Op, redir.Target),
		})
	}

	// 7. Check for unknowns to trigger LLM fallback
	missingItems := detectMissingItems(exp)
	if len(missingItems) > 0 && r.env.LLMClient != nil {
		exp.LLMQueried = true
		exp.LLMEndpoint = r.env.LLMClient.BaseURL()
		var sysCtx llm.SystemContext
		if r.env.LLMEnv != nil {
			sysCtx = llm.GatherContext(r.env.LLMEnv, cmd.Args)
		}
		basicSummary := buildBasicSummary(exp)
		model := r.env.DefaultModel
		if model == "" {
			model = llm.FallbackDefaultModel
		}

		rawCmd := formatRawCommand(cmd)
		chatReq := llm.BuildChatRequest(model, sysCtx, rawCmd, missingItems, basicSummary)

		llmCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		chatResp, err := r.env.LLMClient.Chat(llmCtx, chatReq)
		if err == nil && chatResp.Message.Content != "" {
			parsed := llm.ParseLLMResponse(chatResp.Message.Content)
			exp.LLMExplanation = parsed.Explanation
			exp.SuggestedCommand = parsed.SuggestedCommand
			exp.SuggestedScript = parsed.SuggestedScript
		} else if err != nil {
			exp.LLMError = fmt.Sprintf("LLM server (%s) is not reachable", r.env.LLMClient.BaseURL())
		}
	}

	return exp
}

func (r *Resolver) lookupSummary(ctx context.Context, cmd, subcmd string) string {
	// Try subcommand whatis first e.g. "git-commit"
	if subcmd != "" && r.env.Whatis != nil {
		joined := cmd + "-" + subcmd
		if s, err := r.env.Whatis(ctx, joined); err == nil && s != "" {
			return firstNonEmptyLine(s)
		}
	}

	// Try main command whatis
	if r.env.Whatis != nil {
		if s, err := r.env.Whatis(ctx, cmd); err == nil && s != "" {
			return firstNonEmptyLine(s)
		}
	}

	// Try man page NAME section
	manContent := r.getMan(ctx, cmd, subcmd)
	if manContent != "" {
		if sum := ExtractManSummary(manContent); sum != "" {
			return sum
		}
	}

	return ""
}

func (r *Resolver) lookupFlagDoc(ctx context.Context, cmd, subcmd string, flag Flag) FlagExplanation {
	flagExp := FlagExplanation{
		Flag: flag,
	}

	// 1. Search in `command --help`
	helpText := r.getHelp(ctx, cmd, subcmd)
	if helpText != "" {
		if doc, found := FindOptionInHelp(helpText, flag.Name); found {
			flagExp.Description = doc
			flagExp.Found = true
			flagExp.Source = "help"
			return flagExp
		}
	}

	// 2. Fallback to `man`
	manContent := r.getMan(ctx, cmd, subcmd)
	if manContent != "" {
		if doc, found := FindOptionInMan(manContent, flag.Name); found {
			flagExp.Description = doc
			flagExp.Found = true
			flagExp.Source = "man"
			return flagExp
		}
	}

	return flagExp
}

func (r *Resolver) getHelp(ctx context.Context, cmd, subcmd string) string {
	key := cmd
	if subcmd != "" {
		key = cmd + " " + subcmd
	}
	if cached, ok := r.helpCache[key]; ok {
		return cached
	}

	if r.env.RunCmdTimeout == nil {
		return ""
	}

	timeout := 1 * time.Second

	// If subcommand is present, try `<cmd> <subcmd> --help` or `<cmd> <subcmd> -h`
	if subcmd != "" {
		out, err := r.env.RunCmdTimeout(ctx, timeout, cmd, subcmd, "--help")
		if err == nil && len(out) > 0 {
			r.helpCache[key] = string(out)
			return string(out)
		}
		out, err = r.env.RunCmdTimeout(ctx, timeout, cmd, subcmd, "-h")
		if err == nil && len(out) > 0 {
			r.helpCache[key] = string(out)
			return string(out)
		}
	}

	// Try `<cmd> --help`
	out, err := r.env.RunCmdTimeout(ctx, timeout, cmd, "--help")
	if err == nil && len(out) > 0 {
		r.helpCache[key] = string(out)
		return string(out)
	}

	// Try `<cmd> -h`
	out, err = r.env.RunCmdTimeout(ctx, timeout, cmd, "-h")
	if err == nil && len(out) > 0 {
		r.helpCache[key] = string(out)
		return string(out)
	}

	r.helpCache[key] = ""
	return ""
}

func (r *Resolver) getMan(ctx context.Context, cmd, subcmd string) string {
	key := cmd
	if subcmd != "" {
		key = cmd + "-" + subcmd
	}
	if cached, ok := r.manCache[key]; ok {
		return cached
	}

	if r.env.ManPage == nil {
		return ""
	}

	if subcmd != "" {
		if content, err := r.env.ManPage(ctx, key); err == nil && content != "" {
			r.manCache[key] = content
			return content
		}
	}

	if content, err := r.env.ManPage(ctx, cmd); err == nil && content != "" {
		r.manCache[key] = content
		return content
	}

	r.manCache[key] = ""
	return ""
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func describeOperator(op ControlOperator) string {
	switch op {
	case OpOr:
		return "If the previous command fails (exit code != 0), executes:"
	case OpAnd:
		return "If the previous command succeeds (exit code 0), executes:"
	case OpPipe:
		return "Pipes standard output into:"
	case OpPipeStderr:
		return "Pipes standard output and standard error into:"
	case OpSemicolon:
		return "Then executes:"
	case OpBackground:
		return "Runs in the background, then executes:"
	default:
		return ""
	}
}

func describeRedirect(op, target string) string {
	switch op {
	case ">":
		return fmt.Sprintf("Redirects standard output to %s", target)
	case ">>":
		return fmt.Sprintf("Appends standard output to %s", target)
	case "<":
		return fmt.Sprintf("Reads standard input from %s", target)
	case "2>":
		return fmt.Sprintf("Redirects standard error to %s", target)
	case "2>&1":
		return "Redirects standard error to standard output"
	case "&>":
		return fmt.Sprintf("Redirects standard output and standard error to %s", target)
	default:
		return fmt.Sprintf("Redirect %s %s", op, target)
	}
}

func detectMissingItems(exp *CommandExplanation) []string {
	var missing []string
	if !exp.Found {
		missing = append(missing, fmt.Sprintf("Command %q was not found in system PATH or manual entries", exp.Name))
	}
	for _, flag := range exp.Flags {
		if !flag.Found {
			missing = append(missing, fmt.Sprintf("Option %q was not found in %s documentation", flag.Flag.Name, exp.Name))
		}
	}
	return missing
}

func buildBasicSummary(exp *CommandExplanation) string {
	var sb strings.Builder
	if exp.Summary != "" {
		sb.WriteString(exp.Summary + "\n")
	}
	for _, flag := range exp.Flags {
		if flag.Found {
			sb.WriteString(flag.Flag.Name + ": " + strings.ReplaceAll(flag.Description, "\n", " ") + "\n")
		}
	}
	return sb.String()
}

func formatRawCommand(cmd *Command) string {
	var parts []string
	parts = append(parts, cmd.EnvVars...)
	for _, w := range cmd.Wrappers {
		parts = append(parts, w.Name)
		for _, f := range w.Flags {
			parts = append(parts, f.Raw)
		}
		parts = append(parts, w.Args...)
	}
	if cmd.Name != "" {
		parts = append(parts, cmd.Name)
	}
	if cmd.Subcommand != "" {
		parts = append(parts, cmd.Subcommand)
	}
	for _, f := range cmd.Flags {
		parts = append(parts, f.Raw)
	}
	parts = append(parts, cmd.Args...)
	for _, r := range cmd.Redirects {
		parts = append(parts, r.Op, r.Target)
	}
	return strings.Join(parts, " ")
}
