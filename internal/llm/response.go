package llm

import (
	"regexp"
	"strings"
)

// LLMResult represents parsed sections of the LLM's response.
type LLMResult struct {
	Explanation      string
	SuggestedCommand string
	SuggestedScript  string
}

var suggestedCmdRegex = regexp.MustCompile(`(?i)(?:suggested command|command suggestion|propuesta de comando|comando sugerido):\s*` + "`?" + `([^` + "`" + `\r\n]+)` + "`?")
var codeBlockRegex = regexp.MustCompile("(?s)```(?:bash|sh)?\n(.*?)\n```")

// ParseLLMResponse extracts the explanation, suggested one-liner command,
// and multiline bash script from the model's text output.
func ParseLLMResponse(raw string) LLMResult {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return LLMResult{}
	}

	result := LLMResult{}

	// 1. Extract bash script block if present
	if match := codeBlockRegex.FindStringSubmatch(trimmed); len(match) > 1 {
		script := strings.TrimSpace(match[1])
		if strings.Contains(script, "\n") {
			result.SuggestedScript = script
		} else if result.SuggestedCommand == "" {
			result.SuggestedCommand = script
		}
	}

	// 2. Extract suggested single-line command if present
	if match := suggestedCmdRegex.FindStringSubmatch(trimmed); len(match) > 1 {
		cmd := strings.TrimSpace(match[1])
		cmd = strings.Trim(cmd, "`\"'")
		if cmd != "" && !strings.Contains(cmd, "\n") {
			result.SuggestedCommand = cmd
		}
	}

	// 3. Clean up the explanation text
	explanation := trimmed

	// If explanation contains suggested command line at the end, keep explanation clean
	lines := strings.Split(explanation, "\n")
	var cleanLines []string
	for _, l := range lines {
		trimmedLine := strings.TrimSpace(l)
		if suggestedCmdRegex.MatchString(trimmedLine) {
			continue
		}
		cleanLines = append(cleanLines, l)
	}

	result.Explanation = strings.TrimSpace(strings.Join(cleanLines, "\n"))
	return result
}
