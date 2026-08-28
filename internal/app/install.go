package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"yups/internal/config"
	"yups/internal/llm"
)

// Install implements `yups --install-yups`:
//
//  1. The write-permission probe over the PATH directories runs first,
//     before anything else (acceptance criterion IN-1).
//  2. If the executable is already reachable through one of the PATH
//     directories, the user is informed that it is already installed.
//  3. If a command with the same name exists in one of the well-known
//     system binary directories, the user is informed that it is already
//     installed.
//  4. When several coexisting copies show up anywhere, they are reported
//     together with the first instance found in PATH.
//  5. If the user cannot write in any of the PATH directories: members of
//     an administrator group (sudo, sudoer, sudoers) are suggested to
//     repeat the previous command with sudo (`sudo !!`); anybody else is
//     informed that the installation is not possible.
//  6. Otherwise, the executable is copied into the first writable PATH
//     directory, the ~/.yups configuration directory is initialized,
//     and the user is informed.
func Install(env *Env, stdout, stderr io.Writer) int {
	pathDirs := env.PathDirs()

	// Probed first so the answer is already in hand when it is needed;
	// the already-installed reports below stay correct even when the
	// user cannot write anywhere.
	destDir := firstWritableDir(env, pathDirs)

	if found := findInDirs(env, pathDirs, ProgramName); len(found) > 0 {
		reportInstallAnomaly(env, found, stdout)
		fmt.Fprintf(stdout, "%s is already installed in %s.\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

	if found := findInDirs(env, env.KnownBinDirs(), ProgramName); len(found) > 0 {
		reportInstallAnomaly(env, found, stdout)
		fmt.Fprintf(stdout, "%s is already installed (found outside PATH in %s).\n", ProgramName, quotedJoin(found))
		return ExitOK
	}

	if destDir == "" {
		if isAdmin(env) {
			fmt.Fprintf(stdout,
				"Cannot install %s: none of the PATH directories is writable, but you have administrator privileges; retry the previous command with sudo (sudo !!).\n",
				ProgramName)
		} else {
			fmt.Fprintf(stdout,
				"Cannot install %s: you do not have write permissions on any of the directories where the system stores executables.\n",
				ProgramName)
		}
		return ExitError
	}

	sourcePath, err := env.ExecutablePath()
	if err != nil {
		fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, err)
		return ExitError
	}

	destPath, err := env.InstallTo(sourcePath, destDir)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, err)
		return ExitError
	}

	// Initialize ~/.yups/config.toml on install
	if home, err := env.UserHomeDir(); err == nil {
		cfgPath := config.Path(home)
		cfg, loadErr := env.LoadConfig(cfgPath)
		if loadErr != nil {
			cfg = config.Defaults()
		}
		config.EnsureDefaults(&cfg)
		if cfg.Version == config.FloorVersion || cfg.Version == "" {
			cfg.Version = Version
		}

		hasOllama := true
		if env.AskConfirmation != nil {
			hasOllama = env.AskConfirmation("Do you have an Ollama instance available for AI assistance?", true)
		}

		if !hasOllama {
			cfg.LLMDisabled = true
			fmt.Fprintln(stdout, "Note: AI assistance is disabled (llm-disabled = true). yups will operate in fast local documentation mode (manpages, --help, wrappers, cheatsheets).")
		} else {
			cfg.LLMDisabled = false
			endpoint := cfg.InferenceEndpoint
			if env.AskPrompt != nil {
				endpoint = env.AskPrompt("Ollama inference endpoint", cfg.InferenceEndpoint)
			}
			cfg.InferenceEndpoint = endpoint

			if env.HTTPClient != nil {
				llmClient := llm.NewClient(env.HTTPClient(), endpoint)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				models, err := llmClient.ListModels(ctx)
				cancel()
				if err == nil {
					if len(models) > 0 {
						def, adv := llm.SelectBestModels(models)

						hasQwen := false
						hasGemma := false
						for _, m := range models {
							lower := strings.ToLower(m.Name)
							if strings.Contains(lower, "qwen") {
								hasQwen = true
							}
							if strings.Contains(lower, "gemma") {
								hasGemma = true
							}
						}

						fmt.Fprintf(stdout, "Connected to Ollama at %s (%d models available).\n", endpoint, len(models))

						if env.AskPrompt != nil && (!hasQwen || !hasGemma) {
							fmt.Fprintln(stdout, "\nRecommended models (qwen2.5-coder for default, qwen3.8 for advanced) are not fully available:")
							fmt.Fprintln(stdout, "  [1] Pull recommended models (qwen2.5-coder:7b and qwen3.8:latest)")
							fmt.Fprintln(stdout, "  [2] Choose models from your installed list")
							fmt.Fprintln(stdout, "  [3] Run model benchmark test (--test-models) and choose")
							fmt.Fprintf(stdout, "  [4] Use automatic selection (%s / %s)\n", def, adv)

							choice := strings.TrimSpace(env.AskPrompt("Model setup choice [1/2/3/4]", "4"))
							switch choice {
							case "1":
								fmt.Fprintln(stdout, "Pulling qwen2.5-coder:7b...")
								pullCtx, pullCancel := context.WithTimeout(context.Background(), 10*time.Minute)
								_ = llmClient.PullModel(pullCtx, "qwen2.5-coder:7b", stdout)
								pullCancel()
								def = "qwen2.5-coder:7b"

								fmt.Fprintln(stdout, "Pulling qwen3.8:latest...")
								pullCtx2, pullCancel2 := context.WithTimeout(context.Background(), 10*time.Minute)
								_ = llmClient.PullModel(pullCtx2, "qwen3.8:latest", stdout)
								pullCancel2()
								adv = "qwen3.8:latest"

							case "2":
								def, adv = SelectModelsInteractively(env, models, def, adv, stdout)
							case "3":
								RunModelBenchmark(env, stdout, stderr)
								def, adv = SelectModelsInteractively(env, models, def, adv, stdout)
							case "4", "":
								// use automatic selection
							}
						}

						cfg.DefaultModel = def
						cfg.AdvancedModel = adv
						fmt.Fprintf(stdout, "Configured models: default-model = %s, advanced-model = %s.\n", def, adv)
					} else {
						fmt.Fprintf(stdout, "Connected to Ollama at %s (no models found).\n", endpoint)
						if env.AskConfirmation != nil && env.AskConfirmation("Would you like to pull the recommended qwen2.5-coder:7b model now?", true) {
							fmt.Fprintln(stdout, "Pulling qwen2.5-coder:7b...")
							pullCtx, pullCancel := context.WithTimeout(context.Background(), 10*time.Minute)
							if err := llmClient.PullModel(pullCtx, "qwen2.5-coder:7b", stdout); err == nil {
								cfg.DefaultModel = "qwen2.5-coder:7b"
								cfg.AdvancedModel = "qwen2.5-coder:7b"
							}
							pullCancel()
						}
					}
				} else {
					fmt.Fprintf(stdout, "Ollama is not reachable at %s; yups will operate in basic mode until Ollama is available.\n", endpoint)
				}
			}
		}

		_ = env.SaveConfig(cfgPath, cfg)

		// Download community cheatsheets
		if env.DownloadCheatsheets != nil && env.HTTPClient != nil {
			cheatsDir := config.CheatsheetsDir(home)
			_ = env.DownloadCheatsheets(env.HTTPClient(), cheatsDir, stdout)
		}

		fmt.Fprintf(stdout, "\nConfiguration saved to %s.\n", cfgPath)
		if env.AskConfirmation != nil && env.AskConfirmation("Do you want to review the configuration file?", false) {
			if env.OpenEditor != nil {
				if err := env.OpenEditor(cfgPath, nil, stdout, stderr); err != nil {
					fmt.Fprintf(stderr, "Could not open editor: %v\n", err)
				}
			}
		}
	}

	fmt.Fprintf(stdout, "%s installed in %s.\n", ProgramName, destPath)
	return ExitOK
}

// reportInstallAnomaly warns about several coexisting installations and
// names the chosen target (first instance in PATH).
func reportInstallAnomaly(env *Env, found []string, stdout io.Writer) {
	if len(found) < 2 {
		return
	}
	keeper, others := firstInPath(env, found)
	fmt.Fprintf(stdout,
		"Warning: %s is installed in several places (%s); operating on %s.\n",
		ProgramName, quotedJoin(others), filepath.Join(keeper, ProgramName))
}
