package explain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"yups/internal/cheats"
	"yups/internal/config"
	"yups/internal/llm"
	"yups/internal/sessionlog"
	"yups/internal/ui"
)

// DocEnv defines the OS interface methods needed for documentation retrieval.
type DocEnv struct {
	RunCmdTimeout func(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error)
	Whatis        func(ctx context.Context, cmd string) (string, error)
	ManPage       func(ctx context.Context, cmd string) (string, error)
	TypeCmd       func(ctx context.Context, cmd string) (string, error)
	StatPath      func(path string) (fs.FileInfo, error)
	LookupInPath  func(cmd string) bool

	// CLI flags and logging
	InvocationFlags string
	Logger          *sessionlog.SessionLogger

	// LLM support
	LLMClient            *llm.Client
	LLMEnv               llm.LLMEnv
	DefaultModel         string
	AdvancedModel        string
	OverrideModel        string
	UseAdvanced          bool
	IsInstalled          bool
	LLMEnabled           bool
	LLMTimeout           time.Duration
	ToolExecutionTimeout time.Duration
	MaxToolTurns         int
	MaxToolOutputBytes   int
	AdvancedMultiplier   int
	NoLimits             bool
	ContextLength        int
	IsTerminalOutput     func(w io.Writer) bool
	AskConfirmation      func(prompt string, defaultYes bool) bool
	AskPrompt            func(prompt, defaultValue string) string
	AskEditPrompt        func(prompt, initialValue string) string
	FlushStdin           func()
	ExecShell            func(command string, stdout, stderr io.Writer) int
	CheatsheetsDir       string
	ScriptsDir           string
	OpenEditor           func(path string, stdin io.Reader, stdout, stderr io.Writer) error
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
	if pipeline == nil {
		return &PipelineExplanation{}
	}
	if len(pipeline.Stages) == 0 {
		if pipeline.Comment != "" {
			res := &PipelineExplanation{
				Comment:         pipeline.Comment,
				InvocationFlags: r.env.InvocationFlags,
				RawCommandLine:  "# " + pipeline.Comment,
				HasMissingItems: true,
			}
			if r.env.LLMClient != nil {
				res.LLMEndpoint = r.env.LLMClient.BaseURL()
			}
			if r.env.Logger != nil {
				r.env.Logger.LogSection("LOCAL PIPELINE ANALYSIS")
				r.env.Logger.LogInfo("Comment query: %q, raw command: %q, invocation flags: %q",
					pipeline.Comment, res.RawCommandLine, res.InvocationFlags)
			}
			return res
		}
		return &PipelineExplanation{}
	}

	result := &PipelineExplanation{
		Comment:         pipeline.Comment,
		InvocationFlags: r.env.InvocationFlags,
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

	if r.env.Logger != nil {
		r.env.Logger.LogSection("LOCAL PIPELINE ANALYSIS")
		r.env.Logger.LogInfo("Pipeline stages: %d, comment: %q, raw command: %q, invocation flags: %q",
			len(pipeline.Stages), pipeline.Comment, result.RawCommandLine, result.InvocationFlags)
		for i, st := range result.Stages {
			if st.Command != nil {
				r.env.Logger.LogInfo("Stage %d: cmd=%q found=%t summary=%q flags=%d wrappers=%d",
					i+1, st.Command.Name, st.Command.Found, st.Command.Summary, len(st.Command.Flags), len(st.Command.Wrappers))
			}
		}
		r.env.Logger.LogInfo("Missing items detected: %t", result.HasMissingItems)
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

// ResolveTargetModel resolves the model to use for inference:
// 1. If an override model is given via --model, it is used directly.
// 2. If --advanced or a comment query (# ...) is present, the advanced model is used.
// 3. Otherwise, check Ollama's /api/ps endpoint: if the advanced model is already loaded in memory, use it.
// 4. Otherwise, use the configured default model.
func ResolveTargetModel(ctx context.Context, env DocEnv, isComment bool) (model string, isAdvanced bool) {
	m, adv, _ := ResolveTargetModelWithReason(ctx, env, isComment)
	return m, adv
}

// ResolveTargetModelWithReason returns the chosen model, whether it is advanced, and the selection rationale.
func ResolveTargetModelWithReason(ctx context.Context, env DocEnv, isComment bool) (model string, isAdvanced bool, reason string) {
	if env.OverrideModel != "" {
		return env.OverrideModel, false, fmt.Sprintf("Override model specified via --model=%s", env.OverrideModel)
	}
	if env.UseAdvanced {
		advModel := env.AdvancedModel
		if advModel == "" {
			advModel = llm.FallbackAdvancedModel
		}
		return advModel, true, "Explicit --advanced flag provided"
	}
	if isComment {
		advModel := env.AdvancedModel
		if advModel == "" {
			advModel = llm.FallbackAdvancedModel
		}
		return advModel, true, "Natural language question (# query) routed to advanced model"
	}

	advModel := env.AdvancedModel
	if advModel == "" {
		advModel = llm.FallbackAdvancedModel
	}
	if advModel != "" && env.LLMClient != nil {
		psCtx, psCancel := context.WithTimeout(ctx, 2*time.Second)
		defer psCancel()
		if env.LLMClient.IsModelLoaded(psCtx, advModel) {
			return advModel, true, fmt.Sprintf("Advanced model %s is already loaded in Ollama memory", advModel)
		}
	}

	defModel := env.DefaultModel
	if defModel == "" {
		defModel = llm.FallbackDefaultModel
	}
	return defModel, false, fmt.Sprintf("Using default model %s", defModel)
}

// QueryLLMPipeline executes or continues a conversation with the configured Ollama LLM for a whole pipeline.
func (r *Resolver) QueryLLMPipeline(ctx context.Context, pipeline *Pipeline, exp *PipelineExplanation, userFollowup string, statusWriter io.Writer) error {
	if r.env.LLMClient == nil {
		return fmt.Errorf("no LLM client configured")
	}

	exp.LLMQueried = true
	exp.LLMEndpoint = r.env.LLMClient.BaseURL()

	isComment := pipeline != nil && pipeline.Comment != ""
	model, isAdvanced, modelReason := ResolveTargetModelWithReason(ctx, r.env, isComment)

	if r.env.Logger != nil {
		r.env.Logger.LogConfig(
			r.env.LLMClient.BaseURL(),
			r.env.DefaultModel,
			r.env.AdvancedModel,
			r.env.LLMEnabled,
			r.env.LLMTimeout,
			r.env.ToolExecutionTimeout,
			r.env.MaxToolTurns,
			r.env.MaxToolOutputBytes,
			r.env.AdvancedMultiplier,
			r.env.NoLimits,
		)
		r.env.Logger.LogModelResolution(model, isAdvanced, modelReason)
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
	maxToolTurns := r.env.MaxToolTurns
	if maxToolTurns <= 0 {
		maxToolTurns = 10
	}
	llmTimeout := r.env.LLMTimeout
	if llmTimeout <= 0 {
		llmTimeout = 60 * time.Second
	}
	toolTimeout := r.env.ToolExecutionTimeout
	if toolTimeout <= 0 {
		toolTimeout = 30 * time.Second
	}
	maxOutputBytes := r.env.MaxToolOutputBytes
	if maxOutputBytes <= 0 {
		maxOutputBytes = 4096
	}

	// Advanced model applies multiplier to all limits
	if isAdvanced && !r.env.NoLimits {
		mult := r.env.AdvancedMultiplier
		if mult <= 0 {
			mult = 3
		}
		maxToolTurns *= mult
		llmTimeout *= time.Duration(mult)
		toolTimeout *= time.Duration(mult)
	}

	isTTY := r.env.IsTerminalOutput != nil && statusWriter != nil && r.env.IsTerminalOutput(statusWriter)
	color := isTTY
	executedTools := make(map[string]bool)
	emptyRetryCount := 0

	for turn := 0; r.env.NoLimits || turn < maxToolTurns; turn++ {
		spinMsg := fmt.Sprintf("Querying model (%s)...", model)
		if isAdvanced {
			spinMsg = fmt.Sprintf("Querying advanced model (%s)...", model)
		}
		spinner := ui.StartSpinner(statusWriter, spinMsg, isTTY, color)

		chatStart := time.Now()
		reqCtx, reqCancel := context.WithCancel(ctx)
		type chatResult struct {
			resp llm.ChatResponse
			err  error
		}
		resCh := make(chan chatResult, 1)
		numCtx := r.env.ContextLength
		if numCtx <= 0 {
			numCtx = config.DefaultContextLength
		}
		if r.env.NoLimits && numCtx < 32768 {
			numCtx = 32768
		}
		reqPayload := llm.ChatRequest{
			Model:    model,
			Messages: exp.Conversation,
			Tools:    llm.DefaultTools(),
			Options: map[string]any{
				"temperature": 0.1,
				"num_ctx":     numCtx,
			},
		}
		go func() {
			r, e := r.env.LLMClient.Chat(reqCtx, reqPayload)
			resCh <- chatResult{resp: r, err: e}
		}()

		var resp llm.ChatResponse
		var err error

		if r.env.NoLimits {
			select {
			case <-ctx.Done():
				reqCancel()
				err = ctx.Err()
				goto QueryFinished

			case res := <-resCh:
				reqCancel()
				resp = res.resp
				err = res.err
				goto QueryFinished
			}
		} else {
			timer := time.NewTimer(llmTimeout)
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					reqCancel()
					err = ctx.Err()
					goto QueryFinished

				case res := <-resCh:
					timer.Stop()
					reqCancel()
					resp = res.resp
					err = res.err
					goto QueryFinished

				case <-timer.C:
					spinner.Stop()
					if statusWriter != nil {
						fmt.Fprintf(statusWriter, "\nExecution limit reached: query taking longer than %v (model is still running in background)...\n", llmTimeout)
					}
					if r.env.Logger != nil {
						r.env.Logger.LogIncident("LLM_TIMEOUT", "Turn %d: query exceeded timeout checkpoint %v", turn, llmTimeout)
						r.env.Logger.LogLimitReached("timeout", fmt.Sprintf("exceeded timeout checkpoint %v", llmTimeout), false)
					}

					abort := true
					if r.env.AskConfirmation != nil {
						abort = r.env.AskConfirmation("Do you want to abort execution?", true)
					}
					if r.env.Logger != nil {
						r.env.Logger.LogLimitReached("timeout-prompt", fmt.Sprintf("abort=%t", abort), abort)
					}

					if abort {
						reqCancel()
						if statusWriter != nil {
							fmt.Fprintln(statusWriter, "Execution aborted.")
						}
						exp.LLMError = "execution aborted after timeout"
						if r.env.Logger != nil {
							r.env.Logger.LogIncident("USER_ABORT", "execution aborted by user after LLM timeout")
						}
						return errors.New("execution aborted after timeout")
					}

					// Check if result arrived while user was answering prompt
					select {
					case res := <-resCh:
						reqCancel()
						resp = res.resp
						err = res.err
						goto QueryFinished
					default:
					}

					if statusWriter != nil {
						fmt.Fprintln(statusWriter, "You can stop execution at any time using Ctrl+C or Ctrl+Z.")
					}

					llmTimeout *= 2
					timer.Reset(llmTimeout)
					spinner = ui.StartSpinner(statusWriter, spinMsg, isTTY, color)
				}
			}
		}

	QueryFinished:
		spinner.Stop()
		chatDuration := time.Since(chatStart)

		if r.env.Logger != nil {
			r.env.Logger.LogInteraction(turn, model, r.env.LLMClient.BaseURL(), reqPayload, resp, chatDuration, err)
		}

		if err != nil {
			exp.LLMError = err.Error()
			if r.env.Logger != nil {
				r.env.Logger.LogIncident("LLM_QUERY_ERROR", "Turn %d: inference failed: %v", turn, err)
				r.env.Logger.LogConclusion("", "", "", "ERROR: "+err.Error(), 0)
			}
			return err
		}

		chatResp = resp

		toolCalls := llm.ExtractToolCalls(chatResp.Message)
		contentTrimmed := strings.TrimSpace(chatResp.Message.Content)

		// Handle empty responses from LLM (no content and no tool calls)
		if len(toolCalls) == 0 && contentTrimmed == "" {
			emptyRetryCount++
			if emptyRetryCount < 2 {
				isLengthCutoff := chatResp.DoneReason == "length" || chatResp.Message.Thinking != ""
				extra := ""
				if chatResp.Message.Thinking != "" {
					extra = fmt.Sprintf(" (contained %d bytes of thinking)", len(chatResp.Message.Thinking))
				}
				if chatResp.DoneReason != "" {
					extra += fmt.Sprintf(" [done_reason=%s]", chatResp.DoneReason)
				}
				if r.env.Logger != nil {
					r.env.Logger.LogIncident("EMPTY_RESPONSE_RETRY", "Turn %d: model %s returned empty response%s, retrying...", turn, model, extra)
				}

				var retryPrompt string
				if isLengthCutoff {
					retryPrompt = "Your previous generation was interrupted because the context length limit was reached. Continue from where you left off and finish what you were doing. When finished, output your response in the required JSON format (with \"explanation\", \"suggested-command\", and \"suggested-script\")."
				} else {
					var originalUserInput string
					if len(exp.Conversation) > 1 && exp.Conversation[1].Role == "user" {
						originalUserInput = exp.Conversation[1].Content
					}
					retryPrompt = "Your previous response was completely empty. If you encountered an error, constraint, difficulty, or security restriction that prevents you from completing the task, DO NOT return a blank response. Explain the error, restriction, or problem in the \"explanation\" field (with \"suggested-command\" and \"suggested-script\" left empty)."
					if originalUserInput != "" {
						retryPrompt = fmt.Sprintf("Your previous response was completely empty.\n\nPlease address the user request:\n%s\n\nIf you encountered an error, constraint, difficulty, or security restriction that prevents you from completing the task, DO NOT return a blank response. Explain the error, restriction, or problem in the \"explanation\" field (with \"suggested-command\" and \"suggested-script\" left empty).", originalUserInput)
					}
				}

				assistantTurn := chatResp.Message
				if assistantTurn.Role == "" {
					assistantTurn.Role = "assistant"
				}
				exp.Conversation = append(exp.Conversation,
					assistantTurn,
					llm.Message{
						Role:    "user",
						Content: retryPrompt,
					},
				)
				turn--
				continue
			}
			// Second consecutive empty response
			if r.env.Logger != nil {
				extra := ""
				if chatResp.Message.Thinking != "" {
					extra = fmt.Sprintf(" (contained %d bytes of thinking)", len(chatResp.Message.Thinking))
				}
				if chatResp.DoneReason != "" {
					extra += fmt.Sprintf(" [done_reason=%s]", chatResp.DoneReason)
				}
				r.env.Logger.LogIncident("EMPTY_RESPONSE_ABORT", "Turn %d: model %s returned empty response twice consecutively%s", turn, model, extra)
			}
			if statusWriter != nil {
				if color {
					theme := ui.GetTheme()
					fmt.Fprintf(statusWriter, "\n%sWarning:%s The model %s returned an empty response.\n", theme.Warning, theme.Reset, model)
				} else {
					fmt.Fprintf(statusWriter, "\nWarning: The model %s returned an empty response.\n", model)
				}
			}
			exp.LLMError = "model returned empty response"
			break
		}

		emptyRetryCount = 0

		if len(toolCalls) == 0 {
			break
		}

		// If the model already produced a suggested command or script in its text content,
		// it has formulated its final answer and conclusion. Treat this as the final turn.
		parsed := llm.ParseLLMResponse(chatResp.Message.Content)
		if parsed.SuggestedCommand != "" || parsed.SuggestedScript != "" {
			break
		}

		exp.Conversation = append(exp.Conversation, chatResp.Message)

		gatherDoc := func(cName, sName string) llm.CommandDoc {
			doc := llm.CommandDoc{
				Command:    cName,
				Subcommand: sName,
			}
			help := r.getHelp(ctx, cName, sName)
			if !r.env.NoLimits && len(help) > maxOutputBytes {
				help = help[:maxOutputBytes] + "\n... (truncated)"
			}
			doc.HelpOutput = help

			man := r.getMan(ctx, cName, sName)
			if !r.env.NoLimits {
				maxMan := maxOutputBytes * 3 / 4
				if help != "" {
					maxMan = maxOutputBytes * 3 / 8
				}
				if len(man) > maxMan {
					man = man[:maxMan] + "\n... (truncated)"
				}
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
			key := fmt.Sprintf("%s:%v", tc.Function.Name, tc.Function.Arguments)
			executedTools[key] = true
			switch tc.Function.Name {
			case "fetch-command-documentation":
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

					docStart := time.Now()
					doc := gatherDoc(targetCmd, targetSub)
					docDuration := time.Since(docStart)
					nonce := llm.ExtractNonce(exp.Conversation)
					toolContent := llm.FormatToolResponse(doc, nonce)
					exp.Conversation = append(exp.Conversation, llm.Message{
						Role:    "tool",
						Content: toolContent,
					})

					if r.env.Logger != nil {
						r.env.Logger.LogToolExecution(turn, "fetch-command-documentation", tc.Function.Arguments, true, "documentation", toolContent, docDuration, nil)
					}
				}

			case "command-run", "command_run":
				cmdToRun := ""
				if c, ok := tc.Function.Arguments["command"].(string); ok {
					cmdToRun = strings.TrimSpace(c)
				}

				if cmdToRun != "" {
					nonce := llm.ExtractNonce(exp.Conversation)
					allowed, reason := ValidateWhitelistedCommand(cmdToRun)
					if !allowed {
						if statusWriter != nil {
							fmt.Fprintf(statusWriter, "  LLM requested command '%s' (blocked: %s)\n", cmdToRun, reason)
						}
						var errMsg string
						if nonce != "" {
							errMsg = fmt.Sprintf("<tool-output-%s name=%q command=%q>\nError: Command %q was rejected by whitelist: %s. Only safe inspection commands are permitted.\n</tool-output-%s>", nonce, "command-run", cmdToRun, cmdToRun, reason, nonce)
						} else {
							errMsg = fmt.Sprintf("Error: Command %q was rejected by whitelist: %s. Only safe inspection commands are permitted.", cmdToRun, reason)
						}
						exp.Conversation = append(exp.Conversation, llm.Message{
							Role:    "tool",
							Content: errMsg,
						})
						if r.env.Logger != nil {
							r.env.Logger.LogIncident("TOOL_WHITELIST_REJECTED", "Turn %d: command %q rejected by whitelist: %s", turn, cmdToRun, reason)
							r.env.Logger.LogToolExecution(turn, "command-run", tc.Function.Arguments, false, reason, errMsg, 0, nil)
						}
					} else {
						if statusWriter != nil {
							fmt.Fprintf(statusWriter, "  LLM requested running command: '%s'...\n", cmdToRun)
							fmt.Fprintf(statusWriter, "  Executing whitelisted command...\n")
						}

						execStart := time.Now()
						var cmdTimeout time.Duration
						if !r.env.NoLimits {
							cmdTimeout = toolTimeout
							if cmdTimeout > 15*time.Second {
								cmdTimeout = 15 * time.Second
							}
						}
						output, err := r.execWhitelisted(ctx, cmdToRun, cmdTimeout)
						execDuration := time.Since(execStart)

						isTimeout := !r.env.NoLimits && (errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || (err != nil && (strings.Contains(err.Error(), "killed") || strings.Contains(err.Error(), "signal: interrupt"))))

						if isTimeout {
							if statusWriter != nil {
								fmt.Fprintf(statusWriter, "  Command execution timed out after %v (cancelled).\n", cmdTimeout)
							}
							var outStr string
							if nonce != "" {
								outStr = fmt.Sprintf("<tool-output-%s name=%q command=%q>\nError: Command %q timed out after %v. Inspection commands must complete quickly. Please refine your command with limits (e.g. use maxdepth, head, or specific paths to avoid heavy scans).\n</tool-output-%s>", nonce, "command-run", cmdToRun, cmdToRun, cmdTimeout, nonce)
							} else {
								outStr = fmt.Sprintf("Error: Command %q timed out after %v. Inspection commands must complete quickly. Please refine your command with limits (e.g. use maxdepth, head, or specific paths to avoid heavy scans).", cmdToRun, cmdTimeout)
							}
							exp.Conversation = append(exp.Conversation, llm.Message{
								Role:    "tool",
								Content: outStr,
							})
							if r.env.Logger != nil {
								r.env.Logger.LogIncident("TOOL_TIMEOUT", "Turn %d: command %q timed out after %v", turn, cmdToRun, cmdTimeout)
								r.env.Logger.LogToolExecution(turn, "command-run", tc.Function.Arguments, false, "timeout", outStr, execDuration, err)
							}
						} else {
							if err != nil {
								if len(output) == 0 {
									output = fmt.Appendf([]byte(""), "Error executing command: %v", err)
								}
								if r.env.Logger != nil {
									r.env.Logger.LogIncident("TOOL_ERROR", "Turn %d: command %q failed: %v", turn, cmdToRun, err)
								}
							}
							outStr := string(output)
							if !r.env.NoLimits && len(outStr) > maxOutputBytes {
								outStr = outStr[:maxOutputBytes] + "\n... (truncated)"
							}
							var toolMsgContent string
							if nonce != "" {
								toolMsgContent = fmt.Sprintf("<tool-output-%s name=%q command=%q>\nCommand '%s' output:\n%s\n</tool-output-%s>", nonce, "command-run", cmdToRun, cmdToRun, strings.TrimSpace(outStr), nonce)
							} else {
								toolMsgContent = fmt.Sprintf("Command '%s' output:\n%s", cmdToRun, strings.TrimSpace(outStr))
							}
							exp.Conversation = append(exp.Conversation, llm.Message{
								Role:    "tool",
								Content: toolMsgContent,
							})

							if r.env.Logger != nil {
								r.env.Logger.LogToolExecution(turn, "command-run", tc.Function.Arguments, true, reason, toolMsgContent, execDuration, err)
							}
						}
					}
				}
			}
		}

		// Append a follow-up user turn to ensure Jinja chat templates (e.g. Qwen 3.x / Ollama)
		// that require a user query after tool outputs proceed smoothly without template exceptions.
		nonce := llm.ExtractNonce(exp.Conversation)
		var originalUserInput string
		if len(exp.Conversation) > 1 && exp.Conversation[1].Role == "user" {
			originalUserInput = exp.Conversation[1].Content
		}

		var followupContent string
		if originalUserInput != "" {
			if nonce != "" {
				followupContent = fmt.Sprintf("Here are the tool outputs in <tool-output-%s> above. Continue with your task.\n\nRemember that you can call tools again ('fetch-command-documentation' or 'command-run') if you need more information or inspection.\nWhen you have sufficient information and are ready to provide your final answer, do not call any more tools and output your response in the required JSON format (with \"explanation\", \"suggested-command\", and \"suggested-script\") addressing:\n%s", nonce, originalUserInput)
			} else {
				followupContent = fmt.Sprintf("Here are the tool outputs above. Continue with your task.\n\nRemember that you can call tools again ('fetch-command-documentation' or 'command-run') if you need more information or inspection.\nWhen you have sufficient information and are ready to provide your final answer, do not call any more tools and output your response in the required JSON format (with \"explanation\", \"suggested-command\", and \"suggested-script\") addressing:\n%s", originalUserInput)
			}
		} else {
			if nonce != "" {
				followupContent = fmt.Sprintf("Here are the tool outputs in <tool-output-%s> above. Continue with your task addressing <user-input-%s>.\n\nRemember that you can call tools again if needed. When you have sufficient information and are ready to provide your final answer, do not call any more tools and output your response in the required JSON format (with \"explanation\", \"suggested-command\", and \"suggested-script\").", nonce, nonce)
			} else {
				followupContent = "Here are the tool outputs above. Continue with your task. Remember that you can call tools again if needed. When you have sufficient information and are ready to provide your final answer, do not call any more tools and output your response in the required JSON format (with \"explanation\", \"suggested-command\", and \"suggested-script\")."
			}
		}
		exp.Conversation = append(exp.Conversation, llm.Message{
			Role:    "user",
			Content: followupContent,
		})

		// Check if this was the last allowed tool turn
		if !r.env.NoLimits && turn == maxToolTurns-1 {
			if statusWriter != nil {
				fmt.Fprintf(statusWriter, "\nExecution limit reached: maximum reasoning tool rounds reached (%d turns).\n", maxToolTurns)
			}
			if r.env.Logger != nil {
				r.env.Logger.LogIncident("MAX_TURNS_REACHED", "reached maximum reasoning turns limit (%d turns)", maxToolTurns)
				r.env.Logger.LogLimitReached("max-turns", fmt.Sprintf("reached limit of %d turns", maxToolTurns), false)
			}

			abort := true
			if r.env.AskConfirmation != nil {
				abort = r.env.AskConfirmation("Do you want to abort execution?", true)
			}
			if r.env.Logger != nil {
				r.env.Logger.LogLimitReached("max-turns-prompt", fmt.Sprintf("abort=%t", abort), abort)
			}

			if abort {
				if statusWriter != nil {
					fmt.Fprintln(statusWriter, "Execution aborted.")
				}
				if r.env.Logger != nil {
					r.env.Logger.LogIncident("USER_ABORT", "execution aborted by user after reaching max turns (%d)", maxToolTurns)
				}
				break
			}

			// User decided NOT to abort
			if statusWriter != nil {
				fmt.Fprintln(statusWriter, "You can stop execution at any time using Ctrl+C or Ctrl+Z.")
			}

			if !isAdvanced {
				model = r.env.AdvancedModel
				if model == "" {
					model = llm.FallbackAdvancedModel
				}
				isAdvanced = true
				mult := r.env.AdvancedMultiplier
				if mult <= 0 {
					mult = 3
				}
				maxToolTurns = turn + 1 + (r.env.MaxToolTurns * mult)
				llmTimeout = llmTimeout * time.Duration(mult)
				toolTimeout = toolTimeout * time.Duration(mult)
				if statusWriter != nil {
					fmt.Fprintf(statusWriter, "Switching to advanced reasoning model (%s) to continue analysis (this may take longer due to deeper reasoning)...\n", model)
				}
			} else {
				maxToolTurns = turn + 1 + 5
				if statusWriter != nil {
					fmt.Fprintf(statusWriter, "Continuing with advanced model (%s) for %d additional turns...\n", model, 5)
				}
			}

			exp.Conversation = append(exp.Conversation, llm.Message{
				Role:    "user",
				Content: "Previous turns reached reasoning limit. Please synthesize all gathered documentation and command output, then provide your final explanation and suggested command.",
			})
			continue
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

	if r.env.Logger != nil {
		status := "SUCCESS"
		if exp.LLMError != "" {
			status = "ERROR"
		} else if exp.LLMExplanation == "" && exp.SuggestedCommand == "" {
			status = "INCOMPLETE"
		}
		r.env.Logger.LogConclusion(exp.LLMExplanation, exp.SuggestedCommand, exp.SuggestedScript, status, 0)
	}

	if exp.LLMError != "" {
		return errors.New(exp.LLMError)
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

func (r *Resolver) execWhitelisted(ctx context.Context, cmdStr string, toolTimeout time.Duration) ([]byte, error) {
	if r.env.NoLimits || toolTimeout <= 0 {
		if r.env.RunCmdTimeout != nil {
			return r.env.RunCmdTimeout(ctx, 0, "bash", "-c", cmdStr)
		}
		cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process != nil && cmd.Process.Pid > 0 {
				return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil
		}
		return cmd.CombinedOutput()
	}
	if r.env.RunCmdTimeout != nil {
		return r.env.RunCmdTimeout(ctx, toolTimeout, "bash", "-c", cmdStr)
	}
	execCtx, cancel := context.WithTimeout(ctx, toolTimeout)
	defer cancel()
	cmd := exec.CommandContext(execCtx, "bash", "-c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil && cmd.Process.Pid > 0 {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
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
		sb.WriteString("  ")
		sb.WriteString(exp.AliasInfo)
		sb.WriteString("\n")
	}
	if exp.Summary != "" {
		sb.WriteString("  ")
		sb.WriteString(exp.Summary)
		sb.WriteString("\n")
	}
	for _, flag := range exp.Flags {
		if flag.Found {
			sb.WriteString("  ")
			sb.WriteString(flag.Flag.Name)
			sb.WriteString(":")
			sb.WriteString(strings.ReplaceAll(flag.Description, "\n", " "))
			sb.WriteString("\n")

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
	return strings.Join(parts, " ")
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
			sb.WriteString(" ")
			sb.WriteString(string(stage.Operator))
			sb.WriteString(" ")
		} else if i < len(p.Stages)-1 {
			sb.WriteString(" ")
		}
	}
	if p.Comment != "" {
		cleanComment := strings.TrimLeft(p.Comment, "# ")
		if sb.Len() > 0 {
			sb.WriteString(" # ")
			sb.WriteString(cleanComment)
		} else {
			sb.WriteString("# ")
			sb.WriteString(cleanComment)
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
			sb.WriteString(string(stage.Operator))
			sb.WriteString(" (")
			sb.WriteString(stage.OpSummary)
			sb.WriteString(")\n")
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
