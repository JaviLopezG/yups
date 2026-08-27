package explain

import (
	"fmt"
	"io"
	"strings"
)

// ANSI color escape codes
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiOrange = "\x1b[38;5;214m"
	ansiYellow = "\x1b[1;33m"
	ansiRed    = "\x1b[1;31m"
	ansiCyan   = "\x1b[1;36m"
	ansiGray   = "\x1b[90m"
)

// FormatOptions configures how explanations are formatted.
type FormatOptions struct {
	Color bool
}

// FormatPipeline writes the full pipeline explanation to the given writer.
func FormatPipeline(w io.Writer, p *PipelineExplanation, opts FormatOptions) {
	if p == nil || len(p.Stages) == 0 {
		return
	}

	// 1. Print the #_? logo
	if opts.Color {
		fmt.Fprintln(w, ansiOrange+"#_?"+ansiReset)
	} else {
		fmt.Fprintln(w, "#_?")
	}

	for i, stage := range p.Stages {
		if stage.Command != nil {
			formatCommand(w, stage.Command, opts)
		}

		if stage.Operator != OpNone && stage.OpSummary != "" && i+1 < len(p.Stages) {
			fmt.Fprintln(w)
			if opts.Color {
				fmt.Fprintf(w, "%s%s%s\n", ansiCyan, stage.OpSummary, ansiReset)
			} else {
				fmt.Fprintln(w, stage.OpSummary)
			}
			fmt.Fprintln(w)
		}
	}
}

func formatCommand(w io.Writer, cmd *CommandExplanation, opts FormatOptions) {
	if cmd == nil {
		return
	}

	// 1. Found header
	if cmd.Found {
		if opts.Color {
			fmt.Fprintf(w, "%sFound:%s %s%s%s\n", ansiBold, ansiReset, ansiOrange, cmd.Name, ansiReset)
		} else {
			fmt.Fprintf(w, "Found: %s\n", cmd.Name)
		}
	} else {
		if opts.Color {
			fmt.Fprintf(w, "%sNo manual entry or help found for %q%s\n", ansiRed, cmd.Name, ansiReset)
		} else {
			fmt.Fprintf(w, "No manual entry or help found for %q\n", cmd.Name)
		}
	}

	// 2. Wrappers
	for _, wrapper := range cmd.Wrappers {
		if wrapper.Summary != "" {
			fmt.Fprintf(w, "  Wrapper: %s - %s\n", wrapper.Name, wrapper.Summary)
		} else {
			fmt.Fprintf(w, "  Wrapper: %s\n", wrapper.Name)
		}
		for _, flag := range wrapper.Flags {
			formatFlag(w, flag, opts)
		}
	}

	// 3. Environment variables
	for _, env := range cmd.EnvVars {
		if opts.Color {
			fmt.Fprintf(w, "  %sEnv:%s %s\n", ansiGray, ansiReset, env)
		} else {
			fmt.Fprintf(w, "  Env: %s\n", env)
		}
	}

	// 4. Alias info
	if cmd.AliasInfo != "" {
		fmt.Fprintln(w, cmd.AliasInfo)
	}

	// 5. Builtin info
	if cmd.BuiltinInfo != "" {
		fmt.Fprintln(w, cmd.BuiltinInfo)
	}

	// 6. Summary line
	if cmd.Summary != "" {
		fmt.Fprintln(w, cmd.Summary)
	}

	// 7. Flags
	for _, flag := range cmd.Flags {
		formatFlag(w, flag, opts)
	}

	// 8. Positional arguments
	for _, arg := range cmd.PositionalArgs {
		if arg.Kind == "directory" || arg.Kind == "file" {
			if opts.Color {
				fmt.Fprintf(w, "  %s%s%s (%s)\n", ansiBold, arg.Value, ansiReset, arg.Kind)
			} else {
				fmt.Fprintf(w, "  %s (%s)\n", arg.Value, arg.Kind)
			}
		}
	}

	// 9. Redirects
	for _, redir := range cmd.Redirects {
		if redir.Description != "" {
			fmt.Fprintf(w, "  %s\n", redir.Description)
		}
	}

	// 10. LLM fallback notice & explanation
	if cmd.LLMQueried {
		fmt.Fprintln(w)
		if opts.Color {
			fmt.Fprintf(w, "%sAsking LLM at %s for more information...%s\n", ansiCyan, cmd.LLMEndpoint, ansiReset)
		} else {
			fmt.Fprintf(w, "Asking LLM at %s for more information...\n", cmd.LLMEndpoint)
		}
	}

	if cmd.LLMExplanation != "" {
		fmt.Fprintln(w)
		if opts.Color {
			fmt.Fprintf(w, "%sLLM Explanation:%s\n", ansiCyan, ansiReset)
		} else {
			fmt.Fprintln(w, "LLM Explanation:")
		}
		for _, line := range strings.Split(cmd.LLMExplanation, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	if cmd.SuggestedCommand != "" {
		fmt.Fprintln(w)
		if opts.Color {
			fmt.Fprintf(w, "%sSuggested command:%s\n  \x1b[32m%s\x1b[0m\n", ansiBold, ansiReset, cmd.SuggestedCommand)
		} else {
			fmt.Fprintf(w, "Suggested command:\n  %s\n", cmd.SuggestedCommand)
		}
	}

	if cmd.SuggestedScript != "" {
		fmt.Fprintln(w)
		if opts.Color {
			fmt.Fprintf(w, "%sSuggested script:%s\n", ansiBold, ansiReset)
		} else {
			fmt.Fprintln(w, "Suggested script:")
		}
		fmt.Fprintln(w, "  ```bash")
		for _, line := range strings.Split(cmd.SuggestedScript, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
		fmt.Fprintln(w, "  ```")
	}
}

func formatFlag(w io.Writer, flag FlagExplanation, opts FormatOptions) {
	flagName := flag.Flag.Name
	if flag.Found {
		if opts.Color {
			fmt.Fprintf(w, "%s%s found:%s\n", ansiYellow, flagName, ansiReset)
		} else {
			fmt.Fprintf(w, "%s found:\n", flagName)
		}
		// Indent the description lines
		lines := strings.Split(flag.Description, "\n")
		for _, line := range lines {
			fmt.Fprintf(w, "  %s\n", line)
		}
	} else {
		if opts.Color {
			fmt.Fprintf(w, "%s%s%s: No description found.\n", ansiRed, flagName, ansiReset)
		} else {
			fmt.Fprintf(w, "%s: No description found.\n", flagName)
		}
	}
}
