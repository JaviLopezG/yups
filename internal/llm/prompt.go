package llm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"yups/assets"
)

func generateNonce() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "data"
	}
	return hex.EncodeToString(b)
}

func formatSystemContext(sysCtx SystemContext, nonce string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "<system-context-%s>\n", nonce)
	if sysCtx.CurrentTime != "" {
		fmt.Fprintf(&sb, "  <current-time>%s</current-time>\n", sysCtx.CurrentTime)
	}
	if sysCtx.OSRelease != "" {
		fmt.Fprintf(&sb, "  <os-info>%s</os-info>\n", sysCtx.OSRelease)
	}
	if sysCtx.CWD != "" {
		fmt.Fprintf(&sb, "  <current-working-directory>%s</current-working-directory>\n", sysCtx.CWD)
	}
	if len(sysCtx.CWDListing) > 0 {
		fmt.Fprintf(&sb, "  <current-directory-files>%s</current-directory-files>\n", strings.Join(sysCtx.CWDListing, ", "))
	}
	if len(sysCtx.ParentListing) > 0 {
		fmt.Fprintf(&sb, "  <parent-directory-files>%s</parent-directory-files>\n", strings.Join(sysCtx.ParentListing, ", "))
	}
	if len(sysCtx.RecentHistory) > 0 {
		fmt.Fprintf(&sb, "  <recent-shell-history-%s>\n", nonce)
		for _, h := range sysCtx.RecentHistory {
			if h.Timestamp != "" {
				fmt.Fprintf(&sb, "    <history-entry time=%q>%s</history-entry>\n", h.Timestamp, h.Command)
			} else {
				fmt.Fprintf(&sb, "    <history-entry>%s</history-entry>\n", h.Command)
			}
		}
		fmt.Fprintf(&sb, "  </recent-shell-history-%s>\n", nonce)
	}
	if len(sysCtx.FileSnippets) > 0 {
		fmt.Fprintf(&sb, "  <referenced-file-snippets-%s>\n", nonce)
		for f, snippet := range sysCtx.FileSnippets {
			fmt.Fprintf(&sb, "    <file-snippet path=%q>\n%s\n    </file-snippet>\n", f, snippet)
		}
		fmt.Fprintf(&sb, "  </referenced-file-snippets-%s>\n", nonce)
	}
	fmt.Fprintf(&sb, "</system-context-%s>", nonce)
	return sb.String()
}

// BuildChatRequest constructs a structured ChatRequest containing system context,
// query details, and available documentation tools with dynamic nonce XML tags
// to strictly separate passive untrusted context data from active instructions.
func BuildChatRequest(model string, sysCtx SystemContext, commandLine string, missingItems []string, basicSummary string) ChatRequest {
	nonce := generateNonce()

	template := assets.GetSystemPromptTemplate()
	sysContextStr := formatSystemContext(sysCtx, nonce)
	systemPrompt := strings.ReplaceAll(template, "{{NONCE}}", nonce)
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{SYSTEM_CONTEXT}}", sysContextStr)

	var userSb strings.Builder
	fmt.Fprintf(&userSb, "<user-input-%s>\n%s\n</user-input-%s>\n\n", nonce, commandLine, nonce)

	if basicSummary != "" {
		fmt.Fprintf(&userSb, "<local-documentation-analysis-%s>\n%s\n</local-documentation-analysis-%s>\n\n", nonce, basicSummary, nonce)
	}

	if len(missingItems) > 0 {
		fmt.Fprintf(&userSb, "<unknown-items-%s>\n", nonce)
		for _, item := range missingItems {
			fmt.Fprintf(&userSb, "- %s\n", item)
		}
		fmt.Fprintf(&userSb, "</unknown-items-%s>\n\n", nonce)
	}

	fmt.Fprintf(&userSb, "Task: If needed, invoke tools ('fetch-command-documentation' or 'command-run') to inspect documentation or system state. When finished, output your final answer as a strict JSON object with \"explanation\", \"suggested-command\", and \"suggested-script\" addressing the command, question, or goal in <user-input-%s>.", nonce)

	return ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userSb.String()},
		},
		Tools:   DefaultTools(),
		Stream:  false,
		Options: map[string]any{"temperature": 0.1, "num_ctx": 16384},
	}
}

// ExtractNonce extracts the dynamic XML nonce from the initial messages in a conversation.
func ExtractNonce(messages []Message) string {
	for _, m := range messages {
		if idx := strings.Index(m.Content, "<user-input-"); idx != -1 {
			start := idx + len("<user-input-")
			if end := strings.Index(m.Content[start:], ">"); end != -1 {
				return m.Content[start : start+end]
			}
		}
	}
	return ""
}

// FormatToolResponse formats a CommandDoc into a text message for tool turns.
func FormatToolResponse(doc CommandDoc, nonce string) string {
	var sb strings.Builder
	cmdName := doc.Command
	if doc.Subcommand != "" {
		cmdName += " " + doc.Subcommand
	}

	if nonce != "" {
		fmt.Fprintf(&sb, "<tool-output-%s name=%q command=%q>\n", nonce, "fetch-command-documentation", cmdName)
	}
	fmt.Fprintf(&sb, "Documentation for [%s]:\n", cmdName)
	if doc.HelpOutput != "" {
		fmt.Fprintf(&sb, "\n--- [%s] --help output ---\n%s\n", cmdName, strings.TrimSpace(doc.HelpOutput))
	}
	if doc.ManOutput != "" {
		fmt.Fprintf(&sb, "\n--- [%s] man page ---\n%s\n", cmdName, strings.TrimSpace(doc.ManOutput))
	}
	for _, cs := range doc.Cheatsheets {
		fmt.Fprintf(&sb, "\n--- [%s] %s cheatsheet (%s) ---\n%s\n", cmdName, cs.Source, cs.Name, strings.TrimSpace(cs.Content))
	}
	if doc.HelpOutput == "" && doc.ManOutput == "" && len(doc.Cheatsheets) == 0 {
		fmt.Fprintf(&sb, "\nNo local manual page, --help, or cheatsheets found for %s.\n", cmdName)
	}
	if nonce != "" {
		fmt.Fprintf(&sb, "\n</tool-output-%s>", nonce)
	}
	return strings.TrimSpace(sb.String())
}
