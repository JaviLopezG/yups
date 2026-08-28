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
)

// ModelBenchmarkResult holds the timing and quality evaluation for a single model.
type ModelBenchmarkResult struct {
	Model    string
	Duration time.Duration
	Response string
	Error    string
}

// RunModelBenchmark tests all installed models against a standard prompt
// ("ls -javi && yups -hV"), measuring latency and outputting a summary table
// sorted from fastest to slowest.
func RunModelBenchmark(env *Env, stdout, stderr io.Writer) ([]ModelBenchmarkResult, int) {
	docEnv := env.DocEnv()
	if docEnv.LLMClient == nil {
		fmt.Fprintln(stderr, "Error: No Ollama endpoint is configured. Run 'yups --install-yups' first.")
		return nil, ExitError
	}

	endpoint := docEnv.LLMClient.BaseURL()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	models, err := docEnv.LLMClient.ListModels(ctx)
	cancel()
	if err != nil {
		fmt.Fprintf(stderr, "Error connecting to Ollama at %s: %v\n", endpoint, err)
		return nil, ExitError
	}

	if len(models) == 0 {
		fmt.Fprintf(stdout, "Connected to Ollama at %s, but no models are installed.\n", endpoint)
		fmt.Fprintln(stdout, "Run 'ollama pull qwen2.5-coder:7b' to install a recommended model.")
		return nil, ExitOK
	}

	estSeconds := len(models) * 30
	fmt.Fprintf(stdout, "Discovered %d installed model(s) on Ollama at %s:\n", len(models), endpoint)
	for _, m := range models {
		fmt.Fprintf(stdout, "  - %s\n", m.Name)
	}
	fmt.Fprintf(stdout, "\nBenchmark will test each model with a standard multi-flag query ('ls -javi && yups -hV').\n")
	fmt.Fprintf(stdout, "Estimated duration: ~%d seconds (%d models * ~30s).\n", estSeconds, len(models))

	if env.AskConfirmation != nil {
		if !env.AskConfirmation("Do you want to continue with the model benchmark?", false) {
			fmt.Fprintln(stdout, "Benchmark cancelled.")
			return nil, ExitOK
		}
	}

	testCmdLine := "ls -javi && yups -hV"
	pipeline := explain.Parse([]string{"ls", "-javi", "&&", "yups", "-hV"})

	var results []ModelBenchmarkResult

	fmt.Fprintln(stdout, "\nStarting model benchmark...")
	fmt.Fprintln(stdout, "--------------------------------------------------")

	for i, m := range models {
		fmt.Fprintf(stdout, "\n[%d/%d] Testing model '%s'...\n", i+1, len(models), m.Name)

		modelDocEnv := docEnv
		modelDocEnv.OverrideModel = m.Name
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
			fmt.Fprintf(stdout, "  Status: Failed (%v) [%.2fs]\n", err, elapsed.Seconds())
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
			fmt.Fprintf(stdout, "  Status: Success [%.2fs]\n", elapsed.Seconds())
			if res.Response != "" {
				lines := strings.Split(res.Response, "\n")
				sampleLines := lines
				if len(sampleLines) > 4 {
					sampleLines = sampleLines[:4]
				}
				fmt.Fprintln(stdout, "  Response preview:")
				for _, sl := range sampleLines {
					fmt.Fprintf(stdout, "    %s\n", sl)
				}
				if len(lines) > 4 {
					fmt.Fprintln(stdout, "    ...")
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

	fmt.Fprintln(stdout, "\n==================================================")
	fmt.Fprintln(stdout, "             MODEL BENCHMARK SUMMARY             ")
	fmt.Fprintln(stdout, "==================================================")
	fmt.Fprintf(stdout, "%-35s %-12s %s\n", "Model", "Duration", "Status")
	fmt.Fprintln(stdout, "--------------------------------------------------")
	for _, r := range results {
		status := "OK"
		if r.Error != "" {
			status = "Error"
		}
		fmt.Fprintf(stdout, "%-35s %-12.2fs %s\n", r.Model, r.Duration.Seconds(), status)
	}
	fmt.Fprintln(stdout, "==================================================")

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
