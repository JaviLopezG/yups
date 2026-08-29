package llm

import (
	"encoding/json"
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

// ExtractToolCalls returns all tool calls from a message, whether delivered
// through structured ToolCalls or embedded in plain text output.
func ExtractToolCalls(msg Message) []ToolCall {
	if len(msg.ToolCalls) > 0 {
		return msg.ToolCalls
	}

	raw := strings.TrimSpace(msg.Content)
	if raw == "" {
		return nil
	}

	var calls []ToolCall

	// 1. Check for XML style <tool_call>...</tool_call>
	toolTagRegex := regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)
	if matches := toolTagRegex.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
		for _, m := range matches {
			var rawObj struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
				Function  struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			}
			if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &rawObj); err == nil {
				fnName := rawObj.Name
				args := rawObj.Arguments
				if fnName == "" {
					fnName = rawObj.Function.Name
					args = rawObj.Function.Arguments
				}
				if fnName != "" {
					calls = append(calls, ToolCall{
						Function: ToolCallFunction{
							Name:      fnName,
							Arguments: args,
						},
					})
				}
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// 2. Check for functional text call: fetch-command-documentation(command="...", subcommand="...")
	funcRegex := regexp.MustCompile(`(?i)(?:fetch-command-documentation)\s*\(\s*command\s*=\s*["']([^"']+)["'](?:,\s*subcommand\s*=\s*["']([^"']*)["'])?\s*\)`)
	if matches := funcRegex.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
		for _, m := range matches {
			cmd := m[1]
			subcmd := ""
			if len(m) > 2 {
				subcmd = m[2]
			}
			args := map[string]any{"command": cmd}
			if subcmd != "" {
				args["subcommand"] = subcmd
			}
			calls = append(calls, ToolCall{
				Function: ToolCallFunction{
					Name:      "fetch-command-documentation",
					Arguments: args,
				},
			})
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// 3. Check for functional text call: command-run(command="...") or command_run(command="...")
	cmdRunRegex := regexp.MustCompile(`(?i)(?:command-run|command_run)\s*\(\s*command\s*=\s*["']([^"']+)["']\s*\)`)
	if matches := cmdRunRegex.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
		for _, m := range matches {
			cmd := m[1]
			calls = append(calls, ToolCall{
				Function: ToolCallFunction{
					Name:      "command-run",
					Arguments: map[string]any{"command": cmd},
				},
			})
		}
		if len(calls) > 0 {
			return calls
		}
	}

	return nil
}
