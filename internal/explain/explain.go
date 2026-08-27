package explain

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Explain parses and explains the provided command line arguments, writing
// the result progressively to stdout and providing an interactive execution loop.
func Explain(ctx context.Context, env DocEnv, args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		return 0
	}

	pipeline := Parse(args)
	if len(pipeline.Stages) == 0 {
		return 0
	}

	resolver := NewResolver(env)
	exp := resolver.ExplainPipeline(ctx, pipeline)

	opts := FormatOptions{Color: color}

	// 1. Output basic local analysis immediately
	FormatBasicPipeline(stdout, exp, opts)

	// 2. Identify if any stage has missing items requiring LLM enhancement
	var missingStage *StageExplanation
	var missingCmd *Command
	for i, stage := range exp.Stages {
		if stage.Command != nil && stage.Command.HasMissingItems {
			missingStage = &exp.Stages[i]
			if i < len(pipeline.Stages) {
				missingCmd = pipeline.Stages[i].Command
			}
			break
		}
	}

	if missingStage == nil {
		return 0
	}

	// 3. If LLM is not configured, inform user and exit basic mode
	if env.LLMClient == nil {
		fmt.Fprintln(stdout)
		if !env.IsInstalled {
			fmt.Fprintln(stdout, "Note: yups is running in basic mode because it is not installed or configured yet.")
			fmt.Fprintln(stdout, "Run 'yups --install-yups' to configure an Ollama LLM endpoint (e.g. http://localhost:11434).")
		} else {
			fmt.Fprintln(stdout, "Note: No LLM inference endpoint is configured.")
		}
		return 0
	}

	// 4. Announce LLM query
	endpoint := env.LLMClient.BaseURL()
	FormatLLMNotice(stdout, endpoint, opts)

	// 5. Query LLM
	if err := resolver.QueryLLM(ctx, missingCmd, missingStage.Command, ""); err != nil {
		FormatConnectionError(stdout, endpoint, err, env.IsInstalled, opts)
		return 0
	}

	// 6. Print LLM result
	FormatLLMResult(stdout, missingStage.Command, opts)

	// 7. Interactive execution loop [y/n/e/m]
	if missingStage.Command.SuggestedCommand == "" || env.AskPrompt == nil {
		return 0
	}

	currentCmd := missingStage.Command.SuggestedCommand
	for {
		promptStr := FormatPromptChoice(opts)
		choice := env.AskPrompt(promptStr, "y")
		cleanChoice := strings.ToLower(strings.TrimSpace(choice))

		switch cleanChoice {
		case "y", "yes", "":
			if env.ExecShell != nil {
				fmt.Fprintf(stdout, "\nExecuting: %s\n", currentCmd)
				return env.ExecShell(currentCmd, stdout, stderr)
			}
			return 0
		case "n", "no":
			return 0
		case "e", "edit":
			editPrompt := "Edit command"
			if color {
				editPrompt = "\x1b[1mEdit command\x1b[0m"
			}
			var edited string
			if env.AskEditPrompt != nil {
				edited = env.AskEditPrompt(editPrompt, currentCmd)
			} else if env.AskPrompt != nil {
				edited = env.AskPrompt(editPrompt, currentCmd)
			}
			if strings.TrimSpace(edited) != "" {
				currentCmd = strings.TrimSpace(edited)
			}
			fmt.Fprintf(stdout, "\nUpdated command:\n  %s\n\n", currentCmd)
		case "m", "modifications":
			modPrompt := "Enter modifications or additional context for LLM"
			if color {
				modPrompt = "\x1b[1;36mEnter modifications or additional context for LLM\x1b[0m"
			}
			mod := env.AskPrompt(modPrompt, "")
			if strings.TrimSpace(mod) == "" {
				continue
			}
			FormatLLMNotice(stdout, endpoint, opts)
			if err := resolver.QueryLLM(ctx, missingCmd, missingStage.Command, mod); err != nil {
				FormatConnectionError(stdout, endpoint, err, env.IsInstalled, opts)
				return 0
			}
			FormatLLMResult(stdout, missingStage.Command, opts)
			if missingStage.Command.SuggestedCommand != "" {
				currentCmd = missingStage.Command.SuggestedCommand
			}
		default:
			fmt.Fprintln(stdout, "Please answer y (yes), n (no), e (edit), or m (modifications).")
		}
	}
}
