package app

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"yups/internal/explain"
	"yups/internal/llm"
	"yups/internal/sessionlog"
)

// ModelBenchmarkResult holds the timing and quality evaluation for a single model.
type ModelBenchmarkResult struct {
	Model    string
	Duration time.Duration
	Response string
	Error    string
}

// ANSI escape codes for test-models output
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiOrange = "\x1b[38;5;214m"
	ansiBlue   = "\x1b[38;5;39m"
	ansiCyan   = "\x1b[1;36m"
	ansiGreen  = "\x1b[1;32m"
	ansiRed    = "\x1b[1;31m"
	ansiYellow = "\x1b[1;33m"
	ansiGray   = "\x1b[90m"
)

// RunModelBenchmark tests all installed models against a standard prompt
// ("ls -javi && yups -hV"), measuring latency and outputting a summary table
// sorted from fastest to slowest.
func RunModelBenchmark(env *Env, stdout, stderr io.Writer, logger *sessionlog.SessionLogger) ([]ModelBenchmarkResult, int) {
	docEnv := env.DocEnv()
	if docEnv.LLMClient == nil {
		fmt.Fprintln(stderr, "Error: No Ollama endpoint is configured. Run 'yups --install-yups' first.")
		if logger != nil {
			logger.LogIncident("NO_OLLAMA_ENDPOINT", "No Ollama endpoint configured")
			logger.LogConclusion("", "", "", "NO_OLLAMA_ENDPOINT", ExitError)
		}
		return nil, ExitError
	}

	color := env.IsTerminalOutput != nil && env.IsTerminalOutput(stdout)

	endpoint := docEnv.LLMClient.BaseURL()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	models, err := docEnv.LLMClient.ListModels(ctx)
	cancel()
	if err != nil {
		fmt.Fprintf(stderr, "Error connecting to Ollama at %s: %v\n", endpoint, err)
		if logger != nil {
			logger.LogIncident("LLM_CONNECTION_ERROR", "Error connecting to Ollama at %s: %v", endpoint, err)
			logger.LogConclusion("", "", "", fmt.Sprintf("OLLAMA_CONNECTION_ERROR: %v", err), ExitError)
		}
		return nil, ExitError
	}

	if len(models) == 0 {
		fmt.Fprintf(stdout, "Connected to Ollama at %s, but no models are installed.\n", endpoint)
		fmt.Fprintln(stdout, "Run 'ollama pull qwen2.5-coder:7b' to install a recommended model.")
		if logger != nil {
			logger.LogConclusion("", "", "", "NO_MODELS_INSTALLED", ExitOK)
		}
		return nil, ExitOK
	}

	if logger != nil {
		logger.LogSection("MODEL BENCHMARK START")
		var names []string
		for _, m := range models {
			names = append(names, m.Name)
		}
		logger.LogInfo("Discovered %d installed models: %s", len(models), strings.Join(names, ", "))
	}

	estSeconds := len(models) * 30
	if color {
		fmt.Fprintf(stdout, "%sDiscovered %d installed model(s) on Ollama at %s%s%s:\n", ansiBold, len(models), ansiBlue, endpoint, ansiReset)
		for _, m := range models {
			fmt.Fprintf(stdout, "  - %s%s%s\n", ansiCyan, m.Name, ansiReset)
		}
		fmt.Fprintf(stdout, "\nBenchmark will test each model with standard query: %s%s'ls -javi && yups -hV'%s\n", ansiBold, ansiOrange, ansiReset)
		fmt.Fprintf(stdout, "Estimated duration: ~%d seconds (%d models * ~30s).\n", estSeconds, len(models))
	} else {
		fmt.Fprintf(stdout, "Discovered %d installed model(s) on Ollama at %s:\n", len(models), endpoint)
		for _, m := range models {
			fmt.Fprintf(stdout, "  - %s\n", m.Name)
		}
		fmt.Fprintf(stdout, "\nBenchmark will test each model with standard query: 'ls -javi && yups -hV'\n")
		fmt.Fprintf(stdout, "Estimated duration: ~%d seconds (%d models * ~30s).\n", estSeconds, len(models))
	}

	if env.AskConfirmation != nil {
		if !env.AskConfirmation("Do you want to continue with the model benchmark?", false) {
			fmt.Fprintln(stdout, "Benchmark cancelled.")
			if logger != nil {
				logger.LogConclusion("", "", "", "BENCHMARK_CANCELLED_BY_USER", ExitOK)
			}
			return nil, ExitOK
		}
	}

	testCmdLine := "ls -javi && yups -hV"
	pipeline := explain.Parse([]string{"ls", "-javi", "&&", "yups", "-hV"})

	var results []ModelBenchmarkResult

	if color {
		fmt.Fprintf(stdout, "\n%sStarting model benchmark...%s\n", ansiBold, ansiReset)
		fmt.Fprintln(stdout, ansiGray+"--------------------------------------------------"+ansiReset)
	} else {
		fmt.Fprintln(stdout, "\nStarting model benchmark...")
		fmt.Fprintln(stdout, "--------------------------------------------------")
	}

	for i, m := range models {
		if color {
			fmt.Fprintf(stdout, "\n[%d/%d] Testing model: %s%s%s\n", i+1, len(models), ansiBold+ansiCyan, m.Name, ansiReset)
			fmt.Fprintf(stdout, "  %sTest Query:%s %s%s%s\n", ansiBold, ansiReset, ansiOrange, testCmdLine, ansiReset)
		} else {
			fmt.Fprintf(stdout, "\n[%d/%d] Testing model '%s'...\n", i+1, len(models), m.Name)
			fmt.Fprintf(stdout, "  Test Query: %s\n", testCmdLine)
		}

		if logger != nil {
			logger.LogSection(fmt.Sprintf("BENCHMARK [%d/%d]: %s", i+1, len(models), m.Name))
		}

		modelDocEnv := docEnv
		modelDocEnv.OverrideModel = m.Name
		modelDocEnv.Logger = logger
		resolver := explain.NewResolver(modelDocEnv)

		exp := &explain.PipelineExplanation{
			RawCommandLine: testCmdLine,
		}

		start := time.Now()
		benchCtx, benchCancel := context.WithTimeout(context.Background(), 120*time.Second)
		err := resolver.QueryLLMPipeline(benchCtx, pipeline, exp, "", stdout)
		benchCancel()
		elapsed := time.Since(start)

		res := ModelBenchmarkResult{
			Model:    m.Name,
			Duration: elapsed,
		}

		if err != nil {
			res.Error = err.Error()
			if color {
				fmt.Fprintf(stdout, "  Test Result: %s[FAILED]%s (%v) [%.2fs]\n", ansiBold+ansiRed, ansiReset, err, elapsed.Seconds())
			} else {
				fmt.Fprintf(stdout, "  Status: Failed (%v) [%.2fs]\n", err, elapsed.Seconds())
			}
		} else {
			responseText := exp.LLMExplanation
			if exp.SuggestedCommand != "" {
				if responseText != "" {
					responseText += "\nSuggested command: " + exp.SuggestedCommand
				} else {
					responseText = "Suggested command: " + exp.SuggestedCommand
				}
			}
			res.Response = strings.TrimSpace(responseText)

			if color {
				fmt.Fprintf(stdout, "  Test Result: %s[PASSED]%s (duration: %s%.2fs%s)\n",
					ansiBold+ansiGreen, ansiReset, ansiYellow, elapsed.Seconds(), ansiReset)
				if res.Response != "" {
					fmt.Fprintf(stdout, "  %sResponse to verify acceptability:%s\n", ansiBold+ansiYellow, ansiReset)
					lines := strings.Split(res.Response, "\n")
					sampleLines := lines
					if len(sampleLines) > 6 {
						sampleLines = sampleLines[:6]
					}
					for _, sl := range sampleLines {
						if strings.HasPrefix(sl, "Suggested command:") {
							fmt.Fprintf(stdout, "    %s%s%s\n", ansiBold+ansiGreen, sl, ansiReset)
						} else {
							fmt.Fprintf(stdout, "    %s%s%s\n", ansiGray, sl, ansiReset)
						}
					}
					if len(lines) > 6 {
						fmt.Fprintf(stdout, "    %s...%s\n", ansiGray, ansiReset)
					}
				}
			} else {
				fmt.Fprintf(stdout, "  Status: Success [%.2fs]\n", elapsed.Seconds())
				if res.Response != "" {
					fmt.Fprintln(stdout, "  Response to verify acceptability:")
					lines := strings.Split(res.Response, "\n")
					sampleLines := lines
					if len(sampleLines) > 6 {
						sampleLines = sampleLines[:6]
					}
					for _, sl := range sampleLines {
						fmt.Fprintf(stdout, "    %s\n", sl)
					}
					if len(lines) > 6 {
						fmt.Fprintln(stdout, "    ...")
					}
				}
			}
		}
		results = append(results, res)
	}

	// Sort results by duration (fastest to slowest)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Error != "" && results[j].Error == "" {
			return false
		}
		if results[i].Error == "" && results[j].Error != "" {
			return true
		}
		return results[i].Duration < results[j].Duration
	})

	if color {
		fmt.Fprintln(stdout, "\n"+ansiBold+ansiOrange+"================================================================="+ansiReset)
		fmt.Fprintln(stdout, ansiBold+"                    MODEL BENCHMARK SUMMARY                    "+ansiReset)
		fmt.Fprintf(stdout, "                 %sQuery: '%s'%s\n", ansiOrange, testCmdLine, ansiReset)
		fmt.Fprintln(stdout, ansiBold+ansiOrange+"================================================================="+ansiReset)
		fmt.Fprintf(stdout, "%s%-35s %-12s %s%s\n", ansiBold, "Model", "Duration", "Test Result", ansiReset)
		fmt.Fprintln(stdout, ansiGray+"-----------------------------------------------------------------"+ansiReset)
		for _, r := range results {
			statusStr := ansiBold + ansiGreen + "[PASSED]" + ansiReset
			if r.Error != "" {
				statusStr = ansiBold + ansiRed + "[FAILED]" + ansiReset
			}
			modelStr := ansiCyan + fmt.Sprintf("%-35s", r.Model) + ansiReset
			durStr := fmt.Sprintf("%.2fs", r.Duration.Seconds())
			fmt.Fprintf(stdout, "%s %-12s %s\n", modelStr, durStr, statusStr)
		}
		fmt.Fprintln(stdout, ansiBold+ansiOrange+"================================================================="+ansiReset)
	} else {
		fmt.Fprintln(stdout, "\n=================================================================")
		fmt.Fprintln(stdout, "                    MODEL BENCHMARK SUMMARY                    ")
		fmt.Fprintf(stdout, "                 Query: '%s'\n", testCmdLine)
		fmt.Fprintln(stdout, "=================================================================")
		fmt.Fprintf(stdout, "%-35s %-12s %s\n", "Model", "Duration", "Test Result")
		fmt.Fprintln(stdout, "-----------------------------------------------------------------")
		for _, r := range results {
			status := "[PASSED]"
			if r.Error != "" {
				status = "[FAILED]"
			}
			durStr := fmt.Sprintf("%.2fs", r.Duration.Seconds())
			fmt.Fprintf(stdout, "%-35s %-12s %s\n", r.Model, durStr, status)
		}
		fmt.Fprintln(stdout, "=================================================================")
	}

	if logger != nil {
		logger.LogConclusion("", "", "", "BENCHMARK_COMPLETE", ExitOK)
	}

	return results, ExitOK
}

// SelectModelsInteractively prompts the user to select default and advanced models.
func SelectModelsInteractively(env *Env, models []llm.ModelInfo, currentDefault, currentAdvanced string, stdout io.Writer) (string, string) {
	if len(models) == 0 {
		return currentDefault, currentAdvanced
	}

	fmt.Fprintln(stdout, "\nInstalled Ollama models:")
	for i, m := range models {
		fmt.Fprintf(stdout, "  [%d] %s\n", i+1, m.Name)
	}

	defChoice := currentDefault
	if defChoice == "" {
		defChoice, _ = llm.SelectBestModels(models)
	}
	advChoice := currentAdvanced
	if advChoice == "" {
		_, advChoice = llm.SelectBestModels(models)
	}

	if env.AskPrompt != nil {
		defPrompt := fmt.Sprintf("Select default model (name or number, default: %s)", defChoice)
		input := strings.TrimSpace(env.AskPrompt(defPrompt, defChoice))
		if input != "" {
			defChoice = resolveModelName(input, models, defChoice)
		}

		advPrompt := fmt.Sprintf("Select advanced model (name or number, default: %s)", advChoice)
		inputAdv := strings.TrimSpace(env.AskPrompt(advPrompt, advChoice))
		if inputAdv != "" {
			advChoice = resolveModelName(inputAdv, models, advChoice)
		}
	}

	return defChoice, advChoice
}

func resolveModelName(input string, models []llm.ModelInfo, fallback string) string {
	for i, m := range models {
		if fmt.Sprintf("%d", i+1) == input || m.Name == input {
			return m.Name
		}
	}
	return input
}
