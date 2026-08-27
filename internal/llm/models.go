package llm

import (
	"strings"
)

const (
	FallbackDefaultModel  = "qwen2.5-coder:latest"
	FallbackAdvancedModel = "gemma4:latest"
)

// SelectBestModels evaluates a list of discovered Ollama models and picks
// appropriate default and advanced model identifiers.
func SelectBestModels(models []ModelInfo) (defaultModel, advancedModel string) {
	if len(models) == 0 {
		return FallbackDefaultModel, FallbackAdvancedModel
	}

	var codeModels []ModelInfo
	var generalModels []ModelInfo

	for _, m := range models {
		lower := strings.ToLower(m.Name)
		if strings.Contains(lower, "coder") || strings.Contains(lower, "code") {
			codeModels = append(codeModels, m)
		} else {
			generalModels = append(generalModels, m)
		}
	}

	// 1. Choose default model: prefer code-oriented models
	if len(codeModels) > 0 {
		defaultModel = codeModels[0].Name
	} else if len(generalModels) > 0 {
		defaultModel = generalModels[0].Name
	} else {
		defaultModel = models[0].Name
	}

	// 2. Choose advanced model: look for larger/stronger model or alternative
	if len(models) == 1 {
		advancedModel = defaultModel
		return defaultModel, advancedModel
	}

	// Pick the largest model by byte size or the first non-default model
	var largest ModelInfo
	for _, m := range models {
		if m.Size > largest.Size {
			largest = m
		}
	}

	if largest.Name != "" && largest.Name != defaultModel {
		advancedModel = largest.Name
	} else {
		for _, m := range models {
			if m.Name != defaultModel {
				advancedModel = m.Name
				break
			}
		}
	}

	if advancedModel == "" {
		advancedModel = defaultModel
	}

	return defaultModel, advancedModel
}
