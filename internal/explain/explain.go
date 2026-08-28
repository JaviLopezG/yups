package explain

import (
	"context"
	"fmt"
	"io"
	"strings"

	"yups/internal/llm"
)

// Explain parses and explains the provided command line arguments, writing
// the result progressively to stdout and providing an interactive execution loop.
func Explain(ctx context.Context, env DocEnv, args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		return 0
	}

	pipeline := Parse(args)
	if len(pipeline.Stages) == 0 && pipeline.Comment == "" {
		return 0
	}

	resolver := NewResolver(env)
	exp := resolver.ExplainPipeline(ctx, pipeline)

	opts := FormatOptions{Color: color}

	// 1. Output basic local analysis immediately
	FormatBasicPipeline(stdout, exp, opts)

	// 2. Identify if any part of the pipeline has missing items or comments requiring LLM enhancement
	if !exp.HasMissingItems {
		return 0
	}

	// 3. If LLM is not configured, inform user and exit basic mode
	if env.LLMClient == nil {
		fmt.Fprintln(stdout)
		if !env.IsInstalled {
			fmt.Fprintln(stdout, "Note: yups is running in basic mode because it is not installed or configured yet.")
			if env.AskConfirmation != nil && env.AskConfirmation("Do you want to start the installation process now? (estimated time ~3 minutes)", false) {
				if env.ExecShell != nil {
					return env.ExecShell("yups --install-yups", stdout, stderr)
				}
			}
			fmt.Fprintln(stdout, "Run 'yups --install-yups' to configure an Ollama LLM endpoint (e.g. http://localhost:11434).")
		} else {
			fmt.Fprintln(stdout, "Note: AI assistance is disabled or no LLM endpoint is configured (running in local documentation mode).")
		}
		return 0
	}

	// 4. Announce LLM query
	endpoint := env.LLMClient.BaseURL()
	isComment := pipeline != nil && pipeline.Comment != ""
	model, isAdvanced := ResolveTargetModel(ctx, env, isComment)
	FormatLLMNotice(stdout, endpoint, model, isAdvanced, opts)

	// 5. Query LLM for the whole pipeline
	if err := resolver.QueryLLMPipeline(ctx, pipeline, exp, "", stdout); err != nil {
		FormatConnectionError(stdout, endpoint, err, env.IsInstalled, opts)
		return 0
	}

	// 6. Print LLM result
	FormatLLMPipelineResult(stdout, exp, opts)

	// 7. Interactive execution loop [y/n/e/m]
	if exp.SuggestedCommand == "" || env.AskPrompt == nil {
		return 0
	}

	currentCmd := exp.SuggestedCommand
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
				editPrompt = "\x1b[38;5;214mEdit command\x1b[0m"
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
				modPrompt = "\x1b[38;5;214mEnter modifications or additional context for LLM\x1b[0m"
			}
			mod := env.AskPrompt(modPrompt, "")
			if strings.TrimSpace(mod) == "" {
				continue
			}
			if env.OverrideModel == "" {
				resolver.env.UseAdvanced = true
			}
			modAdv := resolver.env.UseAdvanced
			modModel := env.DefaultModel
			if env.OverrideModel != "" {
				modModel = env.OverrideModel
			} else if modAdv {
				modModel = env.AdvancedModel
				if modModel == "" {
					modModel = llm.FallbackAdvancedModel
				}
			}
			FormatLLMNotice(stdout, endpoint, modModel, modAdv, opts)
			if err := resolver.QueryLLMPipeline(ctx, pipeline, exp, mod, stdout); err != nil {
				FormatConnectionError(stdout, endpoint, err, env.IsInstalled, opts)
				return 0
			}
			FormatLLMPipelineResult(stdout, exp, opts)
			if exp.SuggestedCommand != "" {
				currentCmd = exp.SuggestedCommand
			}
		default:
			fmt.Fprintln(stdout, "Please answer y (yes), n (no), e (edit), or m (modifications).")
		}
	}
}
