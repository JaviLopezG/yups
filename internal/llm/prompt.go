package llm

import (
	"fmt"
	"strings"
)

// BuildChatRequest constructs a lightweight ChatRequest containing system context,
// query details, and available documentation tools.
func BuildChatRequest(model string, sysCtx SystemContext, commandLine string, missingItems []string, basicSummary string) ChatRequest {
	var sysSb strings.Builder
	sysSb.WriteString("You are an expert Linux terminal assistant in the 'yups' CLI.\n")
	sysSb.WriteString("Your goal is to inspect the user's shell command line (which may contain multiple commands chained with &&, ||, |, ;, &).\n\n")

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
	sysSb.WriteString("1. Tool calling:\n")
	sysSb.WriteString("   - 'fetch_command_documentation(command=\"...\", subcommand=\"...\")': fetch manual pages, --help, and cheatsheets for commands.\n")
	sysSb.WriteString("   - 'command-run(command=\"...\")': execute read-only whitelisted shell commands (e.g. ls, pwd, stat, file, du, df, find, locate, tree, cat, head, tail, grep, ps, free, uptime, lscpu, ip, ss, ping, dig, nslookup, etc. combined with standard Bash operators) to inspect system files or state.\n")
	sysSb.WriteString("   You can invoke tools multiple times (at once or across turns).\n")
	sysSb.WriteString("2. Suggestion: If the command line has invalid options, typos, or mistakes, provide the entire corrected command line on a single line:\n")
	sysSb.WriteString("   Suggested command: <full corrected command line>\n")
	sysSb.WriteString("   (If a multiline script is truly necessary, wrap it in a single ```bash ... ``` code block).\n")
	sysSb.WriteString("3. Explanation (STRICT MAXIMUM 256 CHARACTERS):\n")
	sysSb.WriteString("   - If the user asked a question in a comment (e.g. # ¿Cual es la ip...?), provide the direct answer in the explanation.\n")
	sysSb.WriteString("   - If explaining unknown flags or errors, explain ONLY the specific items under 'Unknown items'.\n")
	sysSb.WriteString("   - If everything is understood and there are no questions or unknowns, the explanation can be omitted.\n")
	sysSb.WriteString("   - Do NOT explain known commands, known flags, or shell operators (like &&, ||).\n")
	sysSb.WriteString("   - Do NOT repeat the suggested command inside the explanation; it will already be displayed to the user separately.\n")
	sysSb.WriteString("   - Keep it direct, precise, and under 256 characters without conversational filler.\n")

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

	userSb.WriteString("Task: If needed, invoke tools ('fetch_command_documentation' or 'command-run') to inspect documentation or system state. Provide 'Suggested command: ...' (if a command is needed) and a brief answer or explanation (max 256 chars) covering the question or unknown items.")

	return ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: sysSb.String()},
			{Role: "user", Content: userSb.String()},
		},
		Tools:   DefaultTools(),
		Stream:  false,
		Options: map[string]any{"temperature": 0.1},
	}
}

// FormatToolResponse formats a CommandDoc into a text message for tool turns.
func FormatToolResponse(doc CommandDoc) string {
	var sb strings.Builder
	cmdName := doc.Command
	if doc.Subcommand != "" {
		cmdName += " " + doc.Subcommand
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
	return strings.TrimSpace(sb.String())
}
