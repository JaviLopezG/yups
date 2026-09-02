package explain

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		if env.Logger != nil {
			env.Logger.LogConclusion("", "", "", "LOCAL_DOC_OK", 0)
		}
		return 0
	}

	// 3. If LLM is not configured, inform user and exit basic mode
	if env.LLMClient == nil {
		fmt.Fprintln(stdout)
		if !env.IsInstalled {
			fmt.Fprintln(stdout, "Note: yups is running in basic mode because it is not installed or configured yet.")
			if env.AskConfirmation != nil && env.AskConfirmation("Do you want to start the automatic installation process now? (estimated time ~1-3 minutes)", true) {
				if env.ExecShell != nil {
					if env.Logger != nil {
						env.Logger.LogConclusion("", "", "", "LAUNCH_INSTALL", 0)
					}
					exe, err := os.Executable()
					if err == nil {
						return env.ExecShell(fmt.Sprintf("%q --install-yups", exe), stdout, stderr)
					}
					return env.ExecShell("yups --install-yups", stdout, stderr)
				}
			}
			fmt.Fprintln(stdout, "Run 'yups --install-yups' to configure an Ollama LLM endpoint (e.g. http://localhost:11434).")
		} else {
			fmt.Fprintln(stdout, "Note: AI assistance is disabled or no LLM endpoint is configured (running in local documentation mode).")
		}
		if env.Logger != nil {
			env.Logger.LogConclusion("", "", "", "LLM_DISABLED_OR_NOT_CONFIGURED", 0)
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
		if ctx.Err() != nil {
			return 130
		}
		if env.Logger != nil {
			env.Logger.LogIncident("LLM_CONNECTION_ERROR", "connection failed to %s: %v", endpoint, err)
		}
		FormatConnectionError(stdout, endpoint, err, env.IsInstalled, opts)
		return 0
	}

	// 6. Print LLM result
	FormatLLMPipelineResult(stdout, exp, opts)

	// 7. Interactive execution loop
	if (exp.SuggestedCommand == "" && exp.SuggestedScript == "") || env.AskPrompt == nil {
		return 0
	}

	if env.FlushStdin != nil {
		env.FlushStdin()
	}

	currentCmd := exp.SuggestedCommand
	currentScript := exp.SuggestedScript
	scriptProcessed := (currentScript == "")

	for {
		// 7a. Script interaction
		if !scriptProcessed && currentScript != "" {
			promptStr := FormatPromptChoiceScript(opts)
			choice := env.AskPrompt(promptStr, "e")
			cleanChoice := strings.ToLower(strings.TrimSpace(choice))

			switch cleanChoice {
			case "y", "yes":
				scriptsDir := env.ScriptsDir
				if scriptsDir == "" {
					if home, err := os.UserHomeDir(); err == nil {
						scriptsDir = filepath.Join(home, ".yups", "scripts")
					} else {
						scriptsDir = filepath.Join(os.TempDir(), "yups-scripts")
					}
				}
				_ = os.MkdirAll(scriptsDir, 0o755)
				timestamp := time.Now().Format("2006-01-02-15-04-05")
				scriptPath := filepath.Join(scriptsDir, timestamp+".sh")
				if err := os.WriteFile(scriptPath, []byte(currentScript), 0o755); err != nil {
					fmt.Fprintf(stderr, "Failed to write script: %v\n", err)
					return 1
				}
				_ = os.Setenv("YUPS_SCRIPT", scriptPath)
				if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
					_ = os.WriteFile(marker, []byte("executed\n"), 0o600)
				}
				fmt.Fprintf(stdout, "\nExecuting: %s\n", scriptPath)
				if env.ExecShell != nil {
					return env.ExecShell(fmt.Sprintf("bash %q", scriptPath), stdout, stderr)
				}
				return 0

			case "e", "edit", "":
				scriptsDir := env.ScriptsDir
				if scriptsDir == "" {
					if home, err := os.UserHomeDir(); err == nil {
						scriptsDir = filepath.Join(home, ".yups", "scripts")
					} else {
						scriptsDir = filepath.Join(os.TempDir(), "yups-scripts")
					}
				}
				_ = os.MkdirAll(scriptsDir, 0o755)
				timestamp := time.Now().Format("2006-01-02-15-04-05")
				scriptPath := filepath.Join(scriptsDir, timestamp+".sh")
				if err := os.WriteFile(scriptPath, []byte(currentScript), 0o755); err != nil {
					fmt.Fprintf(stderr, "Failed to write script: %v\n", err)
					return 1
				}
				_ = os.Setenv("YUPS_SCRIPT", scriptPath)
				if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
					_ = os.WriteFile(marker, []byte(scriptPath), 0o600)
				}
				if env.OpenEditor != nil {
					_ = env.OpenEditor(scriptPath, nil, stdout, stderr)
				}
				if data, err := os.ReadFile(scriptPath); err == nil && len(data) > 0 {
					currentScript = string(data)
				}
				fmt.Fprintf(stdout, "\nScript saved to %s (available as $YUPS_SCRIPT)\n\n", scriptPath)
				scriptProcessed = true
				if currentCmd != "" {
					continue
				}
				return 0

			case "n", "no":
				scriptsDir := env.ScriptsDir
				if scriptsDir == "" {
					if home, err := os.UserHomeDir(); err == nil {
						scriptsDir = filepath.Join(home, ".yups", "scripts")
					} else {
						scriptsDir = filepath.Join(os.TempDir(), "yups-scripts")
					}
				}
				_ = os.MkdirAll(scriptsDir, 0o755)
				timestamp := time.Now().Format("2006-01-02-15-04-05")
				scriptPath := filepath.Join(scriptsDir, timestamp+".sh")
				_ = os.WriteFile(scriptPath, []byte(currentScript), 0o755)
				_ = os.Setenv("YUPS_SCRIPT", scriptPath)
				if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
					_ = os.WriteFile(marker, []byte(scriptPath), 0o600)
				}
				fmt.Fprintf(stdout, "\nScript saved to %s (available as $YUPS_SCRIPT)\n\n", scriptPath)
				scriptProcessed = true
				if currentCmd != "" {
					continue
				}
				return 0

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
				currentScript = exp.SuggestedScript
				currentCmd = exp.SuggestedCommand
				scriptProcessed = (currentScript == "")
				continue

			default:
				fmt.Fprintln(stdout, "Please answer y (yes), n (no), e (edit), or m (modifications).")
				continue
			}
		}

		// 7b. Command interaction
		if currentCmd != "" {
			promptStr := FormatPromptChoice(opts)
			choice := env.AskPrompt(promptStr, "y")
			cleanChoice := strings.ToLower(strings.TrimSpace(choice))

			switch cleanChoice {
			case "y", "yes", "":
				if env.ExecShell != nil {
					if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
						_ = os.WriteFile(marker, []byte("executed\n"), 0o600)
					}
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
				continue

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
				currentScript = exp.SuggestedScript
				currentCmd = exp.SuggestedCommand
				scriptProcessed = (currentScript == "")
				continue

			default:
				fmt.Fprintln(stdout, "Please answer y (yes), n (no), e (edit), or m (modifications).")
				continue
			}
		}

		return 0
	}
}
