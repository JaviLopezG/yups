package llm

import (
	"fmt"
	"strings"
)

// BuildChatRequest constructs a ChatRequest containing system context and query details.
func BuildChatRequest(model string, sysCtx SystemContext, commandLine string, missingItems []string, basicSummary string) ChatRequest {
	var sysSb strings.Builder
	sysSb.WriteString("You are an expert Linux terminal assistant in the 'yups' CLI.\n")
	sysSb.WriteString("Your mission is to analyze user shell commands, explain unknown commands or flags, and suggest concise corrections if you detect typos or mistakes.\n\n")

	sysSb.WriteString("System Context:\n")
	if sysCtx.OSRelease != "" {
		fmt.Fprintf(&sysSb, "- OS: %s\n", sysCtx.OSRelease)
	}
	if sysCtx.CWD != "" {
		fmt.Fprintf(&sysSb, "- Current Directory: %s\n", sysCtx.CWD)
	}
	if len(sysCtx.CWDListing) > 0 {
		fmt.Fprintf(&sysSb, "- Files in current directory: %s\n", strings.Join(sysCtx.CWDListing, ", "))
	}
	if len(sysCtx.ParentListing) > 0 {
		fmt.Fprintf(&sysSb, "- Files in parent directory: %s\n", strings.Join(sysCtx.ParentListing, ", "))
	}
	if len(sysCtx.RecentHistory) > 0 {
		sysSb.WriteString("- Recent shell history:\n")
		for _, h := range sysCtx.RecentHistory {
			fmt.Fprintf(&sysSb, "  * %s\n", h)
		}
	}
	if len(sysCtx.FileSnippets) > 0 {
		sysSb.WriteString("- Referenced file snippets:\n")
		for f, snippet := range sysCtx.FileSnippets {
			fmt.Fprintf(&sysSb, "  [%s]:\n%s\n", f, snippet)
		}
	}

	sysSb.WriteString("\nInstructions:\n")
	sysSb.WriteString("1. Briefly explain what the unknown commands, flags, or syntax mean or were likely intended to be.\n")
	sysSb.WriteString("2. If the user command line contains a comment (e.g. #...), treat it as an explicit user question or goal. Assess whether the command achieves that goal, explain any discrepancy, and suggest the ideal command or script to accomplish what the user asked in the comment.\n")
	sysSb.WriteString("3. If you detect a typo, invalid flag, or alternative better command, clearly provide:\n")
	sysSb.WriteString("   Suggested command: <corrected command>\n")
	sysSb.WriteString("   (Or if a multiline script is required, wrap it inside a ```bash ... ``` code block).\n")
	sysSb.WriteString("4. Keep explanations concise, clear, and without unnecessary filler.\n")

	var userSb strings.Builder
	fmt.Fprintf(&userSb, "User command line: %s\n\n", commandLine)

	if basicSummary != "" {
		fmt.Fprintf(&userSb, "Local man/help analysis found:\n%s\n\n", basicSummary)
	}

	if len(missingItems) > 0 {
		userSb.WriteString("Unknown items needing explanation:\n")
		for _, item := range missingItems {
			fmt.Fprintf(&userSb, "- %s\n", item)
		}
		userSb.WriteString("\n")
	}

	userSb.WriteString("Please explain the command/comment and suggest a correction or solution if appropriate.")

	return ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: sysSb.String()},
			{Role: "user", Content: userSb.String()},
		},
		Stream:  false,
		Options: map[string]any{"temperature": 0.1},
	}
}
