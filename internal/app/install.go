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
	"yups/internal/sessionlog"
)

// Install implements `yups --install-yups`:
//
//  1. The write-permission probe over the PATH directories runs first,
//     before anything else (acceptance criterion IN-1).
//  2. If the executable is already reachable through one of the PATH
//     directories, and ~/.yups exists, the user is informed that it is already installed.
//  3. If a command with the same name exists in one of the well-known
//     system binary directories, and ~/.yups exists, the user is informed that it is already
//     installed.
//  4. When several coexisting copies show up anywhere, they are reported
//     together with the first instance found in PATH.
//  5. If the user cannot write in any of the PATH directories: members of
//     an administrator group (sudo, sudoer, sudoers) are suggested to
//     repeat the previous command with sudo (`sudo !!`); anybody else is
//     informed that the installation is not possible.
//  6. Otherwise, the executable is copied into the first writable PATH
//     directory (unless already present in PATH), the ~/.yups configuration
//     directory is initialized, and the user is informed.
func Install(env *Env, stdout, stderr io.Writer) int {
	pathDirs := env.PathDirs()

	// Probed first so the answer is already in hand when it is needed;
	// the already-installed reports below stay correct even when the
	// user cannot write anywhere.
	destDir := firstWritableDir(env, pathDirs)

	yupsDirExists := false
	var userHome string
	if env.UserHomeDir != nil && env.PathExists != nil {
		if home, err := env.UserHomeDir(); err == nil {
			userHome = home
			yupsDirExists = env.PathExists(config.Dir(home)) || env.PathExists(config.Path(home))
		}
	}

	foundInPath := findInDirs(env, pathDirs, ProgramName)
	if len(foundInPath) > 0 && yupsDirExists {
		reportInstallAnomaly(env, foundInPath, stdout)
		if userHome != "" {
			sessionlog.RecordIncident(userHome, "", "yups --install-yups", "INSTALL_ALREADY_INSTALLED", "already installed in %s", quotedJoin(foundInPath))
		}
		fmt.Fprintf(stdout, "%s is already installed in %s.\n", ProgramName, quotedJoin(foundInPath))
		return ExitOK
	}

	foundInKnown := findInDirs(env, env.KnownBinDirs(), ProgramName)
	if len(foundInKnown) > 0 && yupsDirExists {
		reportInstallAnomaly(env, foundInKnown, stdout)
		if userHome != "" {
			sessionlog.RecordIncident(userHome, "", "yups --install-yups", "INSTALL_ALREADY_INSTALLED", "already installed (found outside PATH in %s)", quotedJoin(foundInKnown))
		}
		fmt.Fprintf(stdout, "%s is already installed (found outside PATH in %s).\n", ProgramName, quotedJoin(foundInKnown))
		return ExitOK
	}

	var destPath string
	if len(foundInPath) > 0 {
		reportInstallAnomaly(env, foundInPath, stdout)
		destPath = filepath.Join(foundInPath[0], ProgramName)
	} else {
		if destDir == "" {
			if userHome != "" {
				sessionlog.RecordIncident(userHome, "", "yups --install-yups", "INSTALL_PERMISSION_DENIED", "none of the PATH directories is writable")
			}
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
			if userHome != "" {
				sessionlog.RecordIncident(userHome, "", "yups --install-yups", "INSTALL_ERROR", "cannot determine executable path: %v", err)
			}
			fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, err)
			return ExitError
		}

		var installErr error
		destPath, installErr = env.InstallTo(sourcePath, destDir)
		if installErr != nil {
			if userHome != "" {
				sessionlog.RecordIncident(userHome, "", "yups --install-yups", "INSTALL_ERROR", "cannot install to %s: %v", destDir, installErr)
			}
			fmt.Fprintf(stderr, "Cannot install %s: %v.\n", ProgramName, installErr)
			return ExitError
		}
	}

	// Initialize ~/.yups/config.toml on install
	if home, err := env.UserHomeDir(); err == nil {
		cfgPath := config.Path(home)
		cfg, loadErr := env.LoadConfig(cfgPath)
		if loadErr != nil {
			cfg = config.Defaults()
		}
		config.EnsureDefaults(&cfg)

		hasOllama := true
		if env.AskConfirmation != nil {
			hasOllama = env.AskConfirmation("Do you have an Ollama instance available for AI assistance?", true)
		}

		var discoveredModels []string
		if !hasOllama {
			cfg.Inference.Disabled = true
			fmt.Fprintln(stdout, "Note: AI assistance is disabled (llm-disabled = true). yups will operate in fast local documentation mode (manpages, --help, wrappers, cheatsheets).")
		} else {
			cfg.Inference.Disabled = false
			endpoint := cfg.GetInferenceEndpoint()
			if env.AskPrompt != nil {
				endpoint = env.AskPrompt("Ollama inference endpoint", cfg.GetInferenceEndpoint())
			}
			cfg.Inference.Endpoint = endpoint

			if env.HTTPClient != nil && env.IsTerminalOutput != nil && env.IsTerminalOutput(stdout) {
				endpoint := cfg.GetInferenceEndpoint()
				if endpoint != "" {
					llmClient := llm.NewClient(env.HTTPClient(), endpoint)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					models, err := llmClient.ListModels(ctx)
					cancel()
					if err == nil {
						if len(models) > 0 {
							var modelNames []string
							for _, m := range models {
								modelNames = append(modelNames, m.Name)
							}
							discoveredModels = modelNames

							fmt.Fprintf(stdout, "Connected to Ollama at %s (%d models available).\n", endpoint, len(models))

							def := resolveModelSlot(env, llmClient, "default", config.DefaultModel, "qwen3-coder", models, &discoveredModels, stdout)
							adv := resolveModelSlot(env, llmClient, "advanced", config.DefaultAdvancedModel, "gemma4", models, &discoveredModels, stdout)

							cfg.Inference.DefaultModel = def
							cfg.Inference.AdvancedModel = adv
							fmt.Fprintf(stdout, "Configured models: default-model = %s, advanced-model = %s.\n", def, adv)
						} else {
							fmt.Fprintf(stdout, "Connected to Ollama at %s (no models found).\n", endpoint)
							if env.AskConfirmation != nil && env.AskConfirmation(fmt.Sprintf("Would you like to pull the recommended %s model now?", config.DefaultModel), true) {
								fmt.Fprintf(stdout, "Pulling %s...\n", config.DefaultModel)
								pullCtx, pullCancel := context.WithTimeout(context.Background(), 10*time.Minute)
								if err := llmClient.PullModel(pullCtx, config.DefaultModel, stdout); err == nil {
									cfg.Inference.DefaultModel = config.DefaultModel
									cfg.Inference.AdvancedModel = config.DefaultModel
									discoveredModels = []string{config.DefaultModel}
								}
								pullCancel()
							}
						}
					} else {
						fmt.Fprintf(stdout, "Ollama is not reachable at %s; yups will operate in basic mode until Ollama is available.\n", endpoint)
					}
				}
			}
		}

		// Initialize state.toml with version, cheatsheets, and discovered models
		stateFile := config.StatePath(home)
		state, _ := env.LoadUpdateState(stateFile)
		state.Version = Version
		if state.LastApplied == "" || state.LastApplied == config.FloorVersion {
			state.LastApplied = Version
		}
		if len(discoveredModels) > 0 {
			state.AvailableModels = discoveredModels
		}

		// Download or sync community cheatsheets
		if env.DownloadCheatsheets != nil && env.HTTPClient != nil {
			cheatsDir := config.CheatsheetsDir(home)
			if updated, err := env.DownloadCheatsheets(env.HTTPClient(), cheatsDir, state.Cheatsheets, stdout); err == nil && updated != nil {
				state.Cheatsheets = updated
			}
		}
		_ = env.SaveUpdateState(stateFile, state)

		// Configure bash key binding if desired
		ConfigureBashBindingInteractively(env, home, stdout, stderr)

		// Install utility scripts into ~/.yups/scripts/
		_ = InstallScripts(env, home)

		// Install static data and prompt assets into ~/.yups/data/ and ~/.yups/prompts/
		_ = InstallAssets(env, home)

		_ = env.SaveConfig(cfgPath, cfg)
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
	if env.UserHomeDir != nil {
		if home, err := env.UserHomeDir(); err == nil {
			sessionlog.RecordIncident(home, "", "yups --install-yups", "INSTALL_MULTIPLE_BINARIES", "multiple installations detected (%s)", quotedJoin(found))
		}
	}
	fmt.Fprintf(stdout,
		"Warning: %s is installed in several places (%s); operating on %s.\n",
		ProgramName, quotedJoin(others), filepath.Join(keeper, ProgramName))
}

// findInstalledModel returns the exact model name if target (or target without :latest) is installed.
func findInstalledModel(models []llm.ModelInfo, target string) string {
	targetBase := strings.TrimSuffix(target, ":latest")
	for _, m := range models {
		mBase := strings.TrimSuffix(m.Name, ":latest")
		if m.Name == target || m.Name == targetBase || mBase == targetBase {
			return m.Name
		}
	}
	return ""
}

// findFirstFamilyModel returns the first installed model containing the given family string (case-insensitive).
func findFirstFamilyModel(models []llm.ModelInfo, family string) string {
	familyLower := strings.ToLower(family)
	for _, m := range models {
		lower := strings.ToLower(m.Name)
		if strings.Contains(lower, familyLower) {
			return m.Name
		}
	}
	return ""
}

// resolveModelSlot resolves model selection for a given slot (default or advanced):
// 1. If targetModel from config constant is installed, use it.
// 2. If not, suggest pulling it.
// 3. If user declines, prompt choice from installed models, suggesting the first family match (or none).
func resolveModelSlot(
	env *Env,
	llmClient *llm.Client,
	slotName string,
	targetModel string,
	familyPrefix string,
	models []llm.ModelInfo,
	discoveredModels *[]string,
	stdout io.Writer,
) string {
	// 1. If model constant is installed, use that
	if found := findInstalledModel(models, targetModel); found != "" {
		fmt.Fprintf(stdout, "Using installed %s model: %s.\n", slotName, found)
		return found
	}

	// 2. Target model is not installed. Suggest pulling it.
	shouldPull := false
	if env != nil && env.AskConfirmation != nil {
		shouldPull = env.AskConfirmation(
			fmt.Sprintf("Recommended %s model (%s) is not installed. Would you like to pull it now?", slotName, targetModel),
			true,
		)
	}

	if shouldPull && llmClient != nil {
		fmt.Fprintf(stdout, "Pulling %s...\n", targetModel)
		pullCtx, pullCancel := context.WithTimeout(context.Background(), 15*time.Minute)
		err := llmClient.PullModel(pullCtx, targetModel, stdout)
		pullCancel()
		if err == nil {
			if discoveredModels != nil {
				*discoveredModels = append(*discoveredModels, targetModel)
			}
			return targetModel
		}
		fmt.Fprintf(stdout, "Could not pull %s: %v\n", targetModel, err)
	}

	// 3. If user declined or pull failed, let them choose from installed models.
	suggested := findFirstFamilyModel(models, familyPrefix)

	if env != nil && env.AskPrompt != nil && len(models) > 0 {
		fmt.Fprintf(stdout, "\nInstalled Ollama models for %s model:\n", slotName)
		for i, m := range models {
			mark := ""
			if m.Name == suggested {
				mark = " (suggested)"
			}
			fmt.Fprintf(stdout, "  [%d] %s%s\n", i+1, m.Name, mark)
		}

		promptText := fmt.Sprintf("Select %s model (name or number)", slotName)
		if suggested != "" {
			promptText = fmt.Sprintf("Select %s model (name or number, default: %s)", slotName, suggested)
		}

		choice := strings.TrimSpace(env.AskPrompt(promptText, suggested))
		if choice != "" {
			return resolveModelName(choice, models, choice)
		}
	}

	if suggested != "" {
		return suggested
	}
	if len(models) > 0 {
		return models[0].Name
	}
	return targetModel
}
