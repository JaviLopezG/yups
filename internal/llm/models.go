package llm

import (
	"strings"
)

const (
	FallbackDefaultModel  = "qwen2.5-coder:7b"
	FallbackAdvancedModel = "qwen3.8:latest"
)

// RecommendedDefaultModelNames lists preferred default model prefixes in priority order.
var RecommendedDefaultModelNames = []string{
	"qwen3.8-coder",
	"qwen3-coder",
	"qwen3.5-coder",
	"qwen2.5-coder",
	"qwen2-coder",
	"qwen",
}

// RecommendedAdvancedModelNames lists preferred advanced model prefixes in priority order.
var RecommendedAdvancedModelNames = []string{
	"qwen3.8",
	"qwen3",
	"gemma4",
	"gemma3",
	"gemma2",
	"gemma",
	"codegemma",
}

// SelectBestModels evaluates a list of discovered Ollama models and picks
// appropriate default (tool-capable fast model) and advanced (stronger reasoning model) identifiers.
func SelectBestModels(models []ModelInfo) (defaultModel, advancedModel string) {
	if len(models) == 0 {
		return FallbackDefaultModel, FallbackAdvancedModel
	}

	// 1. Choose default model:
	// Prioritize Qwen family first (tested with tools in Ollama)
	for _, pref := range RecommendedDefaultModelNames {
		for _, m := range models {
			lower := strings.ToLower(m.Name)
			if strings.HasPrefix(lower, pref) || strings.Contains(lower, pref) {
				defaultModel = m.Name
				break
			}
		}
		if defaultModel != "" {
			break
		}
	}

	// If no Qwen model, look for other coding models that are NOT codestral (which lacks tool calling)
	if defaultModel == "" {
		for _, m := range models {
			lower := strings.ToLower(m.Name)
			if (strings.Contains(lower, "coder") || strings.Contains(lower, "code")) && !strings.Contains(lower, "codestral") {
				defaultModel = m.Name
				break
			}
		}
	}

	// If still empty, look for non-codestral general models (llama3, dolphin, phi4, etc.)
	if defaultModel == "" {
		for _, m := range models {
			lower := strings.ToLower(m.Name)
			if !strings.Contains(lower, "codestral") {
				defaultModel = m.Name
				break
			}
		}
	}

	// Fallback to first available model if all are codestral or non-preferred
	if defaultModel == "" {
		defaultModel = models[0].Name
	}

	// 2. Choose advanced model:
	if len(models) == 1 {
		return defaultModel, defaultModel
	}

	// Prioritize Gemma family for advanced reasoning/modifications
	for _, pref := range RecommendedAdvancedModelNames {
		for _, m := range models {
			if m.Name == defaultModel {
				continue
			}
			lower := strings.ToLower(m.Name)
			if strings.HasPrefix(lower, pref) || strings.Contains(lower, pref) {
				advancedModel = m.Name
				break
			}
		}
		if advancedModel != "" {
			break
		}
	}

	// If no Gemma model found, pick the largest model by byte size (or first non-default)
	if advancedModel == "" {
		var largest ModelInfo
		for _, m := range models {
			if m.Name != defaultModel && m.Size > largest.Size {
				largest = m
			}
		}
		if largest.Name != "" {
			advancedModel = largest.Name
		} else {
			for _, m := range models {
				if m.Name != defaultModel {
					advancedModel = m.Name
					break
				}
			}
		}
	}

	if advancedModel == "" {
		advancedModel = defaultModel
	}

	return defaultModel, advancedModel
}
