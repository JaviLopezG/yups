package explain

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"time"

	"yups/internal/cheats"
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
	LLMClient      *llm.Client
	LLMEnv         llm.LLMEnv
	DefaultModel   string
	AdvancedModel  string
	IsInstalled    bool
	AskPrompt      func(prompt, defaultValue string) string
	AskEditPrompt  func(prompt, initialValue string) string
	ExecShell      func(command string, stdout, stderr io.Writer) int
	CheatsheetsDir string
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

// ExplainPipeline resolves local documentation for an entire parsed Pipeline.
func (r *Resolver) ExplainPipeline(ctx context.Context, pipeline *Pipeline) *PipelineExplanation {
	if pipeline == nil || len(pipeline.Stages) == 0 {
		return &PipelineExplanation{}
	}

	result := &PipelineExplanation{
		Comment: pipeline.Comment,
	}
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

	result.RawCommandLine = formatRawPipeline(pipeline)
	missingItems := detectPipelineMissingItems(result)
	result.HasMissingItems = len(missingItems) > 0 || result.Comment != ""
	if r.env.LLMClient != nil {
		result.LLMEndpoint = r.env.LLMClient.BaseURL()
	}

	return result
}

// ExplainCommand resolves local documentation for a single command.
func (r *Resolver) ExplainCommand(ctx context.Context, cmd *Command) *CommandExplanation {
	if cmd == nil {
		return &CommandExplanation{}
	}
	if cmd.Name == "" {
		if cmd.Comment != "" {
			exp := &CommandExplanation{
				Comment:         cmd.Comment,
				HasMissingItems: true,
				RawCommand:      formatRawCommand(cmd),
			}
			if r.env.LLMClient != nil {
				exp.LLMEndpoint = r.env.LLMClient.BaseURL()
			}
			return exp
		}
		return &CommandExplanation{}
	}

	exp := &CommandExplanation{
		Name:       cmd.Name,
		Subcommand: cmd.Subcommand,
		EnvVars:    cmd.EnvVars,
		Comment:    cmd.Comment,
	}

	// 1. Alias & Builtin inspection
	if r.env.TypeCmd != nil {
		if typeOut, err := r.env.TypeCmd(ctx, cmd.Name); err == nil && typeOut != "" {
			trimmed := strings.TrimSpace(typeOut)
			if strings.Contains(trimmed, "is an alias for") || strings.Contains(trimmed, "es un alias") {
				exp.AliasInfo = trimmed
			} else if strings.Contains(trimmed, "is a shell builtin") || strings.Contains(trimmed, "es una función") {
				exp.BuiltinInfo = trimmed
			}
		}
	}

	// 2. Command existence check & summary
	if r.env.LookupInPath != nil && r.env.LookupInPath(cmd.Name) {
		exp.Found = true
	} else if exp.AliasInfo != "" || exp.BuiltinInfo != "" {
		exp.Found = true
	}

	exp.Summary = r.lookupSummary(ctx, cmd.Name, cmd.Subcommand)
	if exp.Summary != "" {
		exp.Found = true
	}

	// 3. Wrappers
	for _, wrapper := range cmd.Wrappers {
		wExp := WrapperExplanation{
			Name: wrapper.Name,
			Args: wrapper.Args,
		}
		wExp.Summary = r.lookupSummary(ctx, wrapper.Name, "")
		for _, f := range wrapper.Flags {
			wExp.Flags = append(wExp.Flags, r.lookupFlagDoc(ctx, wrapper.Name, "", f))
		}
		exp.Wrappers = append(exp.Wrappers, wExp)
	}

	// 4. Flags
	handledFullWords := make(map[string]bool)
	for _, flag := range cmd.Flags {
		if flag.FullWord != "" && handledFullWords[flag.FullWord] {
			continue
		}

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

		flagExp := r.lookupFlagDoc(ctx, cmd.Name, cmd.Subcommand, flag)
		exp.Flags = append(exp.Flags, flagExp)
	}

	// 5. Positional arguments inspection (stat filesystem)
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

	// 7. Record missing items and raw command for LLM query
	missingItems := detectMissingItems(exp)
	exp.HasMissingItems = len(missingItems) > 0 || exp.Comment != ""
	exp.RawCommand = formatRawCommand(cmd)
	if r.env.LLMClient != nil {
		exp.LLMEndpoint = r.env.LLMClient.BaseURL()
	}

	return exp
}

// QueryLLMPipeline executes or continues a conversation with the configured Ollama LLM
// for an entire command pipeline, passing the full command line, composite summaries, and
// supporting on-demand tool calls across all stages.
func (r *Resolver) QueryLLMPipeline(ctx context.Context, pipeline *Pipeline, exp *PipelineExplanation, userFollowup string, statusWriter io.Writer) error {
	if r.env.LLMClient == nil {
		return fmt.Errorf("no LLM client configured")
	}

	exp.LLMQueried = true
	exp.LLMEndpoint = r.env.LLMClient.BaseURL()

	model := r.env.DefaultModel
	if model == "" {
		model = llm.FallbackDefaultModel
	}

	if len(exp.Conversation) == 0 {
		var allArgs []string
		if pipeline != nil {
			for _, st := range pipeline.Stages {
				if st.Command != nil {
					allArgs = append(allArgs, st.Command.Args...)
				}
			}
		}
		var sysCtx llm.SystemContext
		if r.env.LLMEnv != nil {
			sysCtx = llm.GatherContext(r.env.LLMEnv, allArgs)
		}
		basicSummary := buildPipelineBasicSummary(exp)
		missingItems := detectPipelineMissingItems(exp)
		chatReq := llm.BuildChatRequest(model, sysCtx, exp.RawCommandLine, missingItems, basicSummary)
		exp.Conversation = chatReq.Messages
	} else if userFollowup != "" {
		exp.Conversation = append(exp.Conversation, llm.Message{
			Role:    "user",
			Content: userFollowup,
		})
	}

	var chatResp llm.ChatResponse
	maxToolTurns := 10

	for turn := 0; turn < maxToolTurns; turn++ {
		turnCtx, turnCancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := r.env.LLMClient.Chat(turnCtx, llm.ChatRequest{
			Model:    model,
			Messages: exp.Conversation,
			Tools:    llm.DefaultTools(),
		})
		turnCancel()
		if err != nil {
			exp.LLMError = err.Error()
			return err
		}
		chatResp = resp

		toolCalls := llm.ExtractToolCalls(chatResp.Message)
		if len(toolCalls) == 0 {
			break
		}

		exp.Conversation = append(exp.Conversation, chatResp.Message)

		gatherDoc := func(cName, sName string) llm.CommandDoc {
			doc := llm.CommandDoc{
				Command:    cName,
				Subcommand: sName,
			}
			help := r.getHelp(ctx, cName, sName)
			if len(help) > 4096 {
				help = help[:4096] + "\n... (truncated)"
			}
			doc.HelpOutput = help

			man := r.getMan(ctx, cName, sName)
			maxMan := 3072
			if help != "" {
				maxMan = 1536
			}
			if len(man) > maxMan {
				man = man[:maxMan] + "\n... (truncated)"
			}
			doc.ManOutput = man

			if r.env.CheatsheetsDir != "" {
				entries := cheats.FindCheatsheets(r.env.CheatsheetsDir, cName, sName)
				for _, e := range entries {
					doc.Cheatsheets = append(doc.Cheatsheets, llm.CheatsheetDoc{
						Source:  e.Source,
						Name:    e.Name,
						Content: e.Content,
					})
				}
			}
			return doc
		}

		for _, tc := range toolCalls {
			switch tc.Function.Name {
			case "fetch_command_documentation":
				targetCmd := ""
				targetSub := ""
				if c, ok := tc.Function.Arguments["command"].(string); ok {
					targetCmd = c
				}
				if s, ok := tc.Function.Arguments["subcommand"].(string); ok {
					targetSub = s
				}

				if targetCmd != "" {
					if statusWriter != nil {
						displayName := targetCmd
						if targetSub != "" {
							displayName += " " + targetSub
						}
						fmt.Fprintf(statusWriter, "  LLM requested detailed documentation for '%s'...\n", displayName)
						fmt.Fprintf(statusWriter, "  Gathering manual pages, --help, and cheatsheets for '%s'...\n", displayName)
					}

					doc := gatherDoc(targetCmd, targetSub)
					toolContent := llm.FormatToolResponse(doc)
					exp.Conversation = append(exp.Conversation, llm.Message{
						Role:    "tool",
						Content: toolContent,
					})
				}

			case "command-run", "command_run":
				cmdToRun := ""
				if c, ok := tc.Function.Arguments["command"].(string); ok {
					cmdToRun = strings.TrimSpace(c)
				}

				if cmdToRun != "" {
					allowed, reason := ValidateWhitelistedCommand(cmdToRun)
					if !allowed {
						if statusWriter != nil {
							fmt.Fprintf(statusWriter, "  LLM requested command '%s' (blocked: %s)\n", cmdToRun, reason)
						}
						exp.Conversation = append(exp.Conversation, llm.Message{
							Role:    "tool",
							Content: fmt.Sprintf("Error: Command %q was rejected by whitelist: %s. Only safe inspection commands are permitted.", cmdToRun, reason),
						})
					} else {
						if statusWriter != nil {
							fmt.Fprintf(statusWriter, "  LLM requested running command: '%s'...\n", cmdToRun)
							fmt.Fprintf(statusWriter, "  Executing whitelisted command...\n")
						}

						output, err := r.execWhitelisted(ctx, cmdToRun)
						if err != nil && len(output) == 0 {
							output = []byte(fmt.Sprintf("Error executing command: %v", err))
						}
						outStr := string(output)
						if len(outStr) > 4096 {
							outStr = outStr[:4096] + "\n... (truncated)"
						}
						exp.Conversation = append(exp.Conversation, llm.Message{
							Role:    "tool",
							Content: fmt.Sprintf("Command '%s' output:\n%s", cmdToRun, strings.TrimSpace(outStr)),
						})
					}
				}
			}
		}

		// If this was the last allowed tool turn, request the final conclusion from the model
		if turn == maxToolTurns-1 {
			finalCtx, finalCancel := context.WithTimeout(ctx, 60*time.Second)
			finalResp, finalErr := r.env.LLMClient.Chat(finalCtx, llm.ChatRequest{
				Model:    model,
				Messages: exp.Conversation,
			})
			finalCancel()
			if finalErr == nil && finalResp.Message.Content != "" {
				chatResp = finalResp
			}
		}
	}

	if chatResp.Message.Content != "" {
		exp.Conversation = append(exp.Conversation, chatResp.Message)
		parsed := llm.ParseLLMResponse(chatResp.Message.Content)
		exp.LLMExplanation = parsed.Explanation
		exp.SuggestedCommand = parsed.SuggestedCommand
		exp.SuggestedScript = parsed.SuggestedScript
		exp.LLMError = ""
	}

	return nil
}

// QueryLLM executes or continues a conversation with the configured Ollama LLM for a single command.
func (r *Resolver) QueryLLM(ctx context.Context, cmd *Command, exp *CommandExplanation, userFollowup string, statusWriter io.Writer) error {
	p := &Pipeline{
		Stages: []Stage{
			{Command: cmd},
		},
		Comment: exp.Comment,
	}
	pExp := &PipelineExplanation{
		Stages: []StageExplanation{
			{Command: exp},
		},
		Comment:         exp.Comment,
		HasMissingItems: exp.HasMissingItems,
		RawCommandLine:  exp.RawCommand,
		Conversation:    exp.Conversation,
	}
	err := r.QueryLLMPipeline(ctx, p, pExp, userFollowup, statusWriter)
	exp.Conversation = pExp.Conversation
	exp.LLMQueried = pExp.LLMQueried
	exp.LLMEndpoint = pExp.LLMEndpoint
	exp.LLMExplanation = pExp.LLMExplanation
	exp.SuggestedCommand = pExp.SuggestedCommand
	exp.SuggestedScript = pExp.SuggestedScript
	exp.LLMError = pExp.LLMError
	return err
}

func (r *Resolver) execWhitelisted(ctx context.Context, cmdStr string) ([]byte, error) {
	if r.env.RunCmdTimeout != nil {
		return r.env.RunCmdTimeout(ctx, 10*time.Second, "bash", "-c", cmdStr)
	}
	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(execCtx, "bash", "-c", cmdStr)
	return cmd.CombinedOutput()
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
	if exp == nil {
		return ""
	}
	var sb strings.Builder
	if exp.AliasInfo != "" {
		sb.WriteString("  " + exp.AliasInfo + "\n")
	}
	if exp.Summary != "" {
		sb.WriteString("  " + exp.Summary + "\n")
	}
	for _, flag := range exp.Flags {
		if flag.Found {
			sb.WriteString("  " + flag.Flag.Name + ": " + strings.ReplaceAll(flag.Description, "\n", " ") + "\n")
		}
	}
	return sb.String()
}

func formatRawCommand(cmd *Command) string {
	var parts []string
	parts = append(parts, cmd.EnvVars...)
	for _, w := range cmd.Wrappers {
		parts = append(parts, w.Name)
		parts = appendFlagsRaw(parts, w.Flags)
		parts = append(parts, w.Args...)
	}
	if cmd.Name != "" {
		parts = append(parts, cmd.Name)
	}
	if cmd.Subcommand != "" {
		parts = append(parts, cmd.Subcommand)
	}
	parts = appendFlagsRaw(parts, cmd.Flags)
	parts = append(parts, cmd.Args...)
	for _, r := range cmd.Redirects {
		parts = append(parts, r.Op, r.Target)
	}
	res := strings.Join(parts, " ")
	if cmd.Comment != "" {
		if res != "" {
			res += " #" + cmd.Comment
		} else {
			res = "#" + cmd.Comment
		}
	}
	return res
}

func appendFlagsRaw(parts []string, flags []Flag) []string {
	seen := make(map[string]bool)
	for _, f := range flags {
		if f.IsClustered {
			if !seen[f.Raw] {
				seen[f.Raw] = true
				parts = append(parts, f.Raw)
			}
		} else {
			parts = append(parts, f.Raw)
		}
	}
	return parts
}

func formatRawPipeline(p *Pipeline) string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	for i, stage := range p.Stages {
		if stage.Command != nil {
			sb.WriteString(formatRawCommand(stage.Command))
		}
		if stage.Operator != OpNone {
			sb.WriteString(" " + string(stage.Operator) + " ")
		} else if i < len(p.Stages)-1 {
			sb.WriteString(" ")
		}
	}
	if p.Comment != "" {
		if sb.Len() > 0 {
			sb.WriteString(" #" + p.Comment)
		} else {
			sb.WriteString("#" + p.Comment)
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildPipelineBasicSummary(exp *PipelineExplanation) string {
	if exp == nil {
		return ""
	}
	var sb strings.Builder
	for i, stage := range exp.Stages {
		if stage.Command != nil {
			cmdExp := stage.Command
			cmdName := cmdExp.Name
			if cmdExp.Subcommand != "" {
				cmdName += " " + cmdExp.Subcommand
			}
			if len(exp.Stages) > 1 && cmdName != "" {
				fmt.Fprintf(&sb, "Command %d [%s]:\n", i+1, cmdName)
			}
			sb.WriteString(buildBasicSummary(cmdExp))
		}
		if stage.Operator != OpNone {
			sb.WriteString(string(stage.Operator) + " (" + stage.OpSummary + ")\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

func detectPipelineMissingItems(exp *PipelineExplanation) []string {
	if exp == nil {
		return nil
	}
	var missing []string
	for _, stage := range exp.Stages {
		if stage.Command != nil {
			cmdExp := stage.Command
			prefix := ""
			if len(exp.Stages) > 1 && cmdExp.Name != "" {
				prefix = fmt.Sprintf("[%s] ", cmdExp.Name)
			}
			if !cmdExp.Found && cmdExp.Name != "" {
				missing = append(missing, fmt.Sprintf("%sCommand %q was not found in system PATH or manual entries", prefix, cmdExp.Name))
			}
			for _, flag := range cmdExp.Flags {
				if !flag.Found {
					missing = append(missing, fmt.Sprintf("%sOption %q was not found in %s documentation", prefix, flag.Flag.Name, cmdExp.Name))
				}
			}
		}
	}
	return missing
}
