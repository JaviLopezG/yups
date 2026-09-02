package explain

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"yups/internal/ui"
)

// GetTerminalHeight returns the current terminal row count, or 24 as fallback default.
func GetTerminalHeight() int {
	ws := &struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}{}
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stderr.Fd(), os.Stdin.Fd()} {
		_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
		if err == 0 && ws.Row > 0 {
			return int(ws.Row)
		}
	}
	if linesStr := os.Getenv("LINES"); linesStr != "" {
		if l, err := strconv.Atoi(linesStr); err == nil && l > 0 {
			return l
		}
	}
	return 24
}

// logoColor is the brand colour for the yups logo #_?.
const logoColor = "\x1b[38;5;214m"

// FormatOptions configures how explanations are formatted.
type FormatOptions struct {
	Color bool
}

// FormatPipeline writes the full pipeline explanation to the given writer.
func FormatPipeline(w io.Writer, p *PipelineExplanation, opts FormatOptions) {
	FormatBasicPipeline(w, p, opts)
	for _, stage := range p.Stages {
		if stage.Command != nil {
			FormatLLMResult(w, stage.Command, opts)
		}
	}
}

// FormatInvocationHeader formats the #_? logo line with cyan flags and gray command line.
func FormatInvocationHeader(w io.Writer, flags, cmd string, color bool) {
	flags = strings.TrimSpace(flags)
	cmd = strings.TrimSpace(cmd)
	theme := ui.GetTheme()

	if color {
		if flags != "" && cmd != "" {
			fmt.Fprintf(w, "%s#_?%s %s%s%s %s%s%s\n", logoColor, theme.Reset, theme.Info, flags, theme.Reset, theme.Muted, cmd, theme.Reset)
		} else if flags != "" {
			fmt.Fprintf(w, "%s#_?%s %s%s%s\n", logoColor, theme.Reset, theme.Info, flags, theme.Reset)
		} else if cmd != "" {
			fmt.Fprintf(w, "%s#_?%s %s%s%s\n", logoColor, theme.Reset, theme.Muted, cmd, theme.Reset)
		} else {
			fmt.Fprintln(w, logoColor+"#_?"+theme.Reset)
		}
	} else {
		if flags != "" && cmd != "" {
			fmt.Fprintf(w, "#_? %s %s\n", flags, cmd)
		} else if flags != "" {
			fmt.Fprintf(w, "#_? %s\n", flags)
		} else if cmd != "" {
			fmt.Fprintf(w, "#_? %s\n", cmd)
		} else {
			fmt.Fprintln(w, "#_?")
		}
	}
}

// FormatBasicPipeline writes the basic local documentation for all pipeline stages.
func FormatBasicPipeline(w io.Writer, p *PipelineExplanation, opts FormatOptions) {
	if p == nil {
		return
	}

	FormatInvocationHeader(w, p.InvocationFlags, p.RawCommandLine, opts.Color)
	theme := ui.GetTheme()

	for i, stage := range p.Stages {
		if stage.Command != nil {
			FormatBasicCommand(w, stage.Command, opts)
		}

		if stage.Operator != OpNone && stage.OpSummary != "" && i+1 < len(p.Stages) {
			fmt.Fprintln(w)
			if opts.Color {
				fmt.Fprintf(w, "%s%s%s\n", theme.Info, stage.OpSummary, theme.Reset)
			} else {
				fmt.Fprintln(w, stage.OpSummary)
			}
			fmt.Fprintln(w)
		}
	}
}

// FormatBasicCommand formats the locally resolved manual/help information.
func FormatBasicCommand(w io.Writer, cmd *CommandExplanation, opts FormatOptions) {
	if cmd == nil {
		return
	}

	theme := ui.GetTheme()

	// 1. Found header
	if cmd.Name != "" {
		if cmd.Found {
			if opts.Color {
				fmt.Fprintf(w, "%sFound:%s %s%s%s\n", theme.Bold, theme.Reset, theme.Important, cmd.Name, theme.Reset)
			} else {
				fmt.Fprintf(w, "Found: %s\n", cmd.Name)
			}
		} else {
			if opts.Color {
				fmt.Fprintf(w, "%sNo manual entry or help found for %q%s\n", theme.Error, cmd.Name, theme.Reset)
			} else {
				fmt.Fprintf(w, "No manual entry or help found for %q\n", cmd.Name)
			}
		}
	}

	// 2. Wrappers
	for _, wrapper := range cmd.Wrappers {
		if opts.Color {
			if wrapper.Summary != "" {
				fmt.Fprintf(w, "  %sWrapper:%s %s%s%s - %s\n", theme.Bold, theme.Reset, theme.Important, wrapper.Name, theme.Reset, wrapper.Summary)
			} else {
				fmt.Fprintf(w, "  %sWrapper:%s %s%s%s\n", theme.Bold, theme.Reset, theme.Important, wrapper.Name, theme.Reset)
			}
		} else {
			if wrapper.Summary != "" {
				fmt.Fprintf(w, "  Wrapper: %s - %s\n", wrapper.Name, wrapper.Summary)
			} else {
				fmt.Fprintf(w, "  Wrapper: %s\n", wrapper.Name)
			}
		}
		for _, flag := range wrapper.Flags {
			formatFlag(w, flag, opts)
		}
	}

	// 3. Environment variables
	for _, env := range cmd.EnvVars {
		if opts.Color {
			fmt.Fprintf(w, "  %sEnv:%s %s\n", theme.Muted, theme.Reset, env)
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
				fmt.Fprintf(w, "  %s%s%s (%s)\n", theme.Bold, arg.Value, theme.Reset, arg.Kind)
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

	// 10. Comment
	if cmd.Comment != "" {
		if opts.Color {
			fmt.Fprintf(w, "  %s# %s%s\n", theme.Muted, cmd.Comment, theme.Reset)
		} else {
			fmt.Fprintf(w, "  # %s\n", cmd.Comment)
		}
	}
}

// FormatLLMNotice prints the query announcement before calling the LLM.
func FormatLLMNotice(w io.Writer, endpoint, model string, isAdvanced bool, opts FormatOptions) {
	fmt.Fprintln(w)
	var msg string
	if isAdvanced {
		if model != "" {
			msg = fmt.Sprintf("Asking advanced LLM (%s) at %s for more information (this may take longer)...", model, endpoint)
		} else {
			msg = fmt.Sprintf("Asking advanced LLM at %s for more information (this may take longer)...", endpoint)
		}
	} else {
		if model != "" {
			msg = fmt.Sprintf("Asking LLM (%s) at %s for more information...", model, endpoint)
		} else {
			msg = fmt.Sprintf("Asking LLM at %s for more information...", endpoint)
		}
	}

	if opts.Color {
		theme := ui.GetTheme()
		fmt.Fprintf(w, "%s%s%s\n", theme.Info, msg, theme.Reset)
	} else {
		fmt.Fprintln(w, msg)
	}
}

// FormatConnectionError prints detailed connection error info and installation hints.
func FormatConnectionError(w io.Writer, endpoint string, err error, isInstalled bool, opts FormatOptions) {
	fmt.Fprintf(w, "  Cannot connect to Ollama at %s (%v).\n", endpoint, err)
	if !isInstalled {
		fmt.Fprintln(w, "  Note: yups is using default settings because it is not installed or configured yet.")
		fmt.Fprintln(w, "  Run 'yups --install-yups' to configure your Ollama endpoint (e.g. http://marvin:11434).")
	} else {
		fmt.Fprintf(w, "  Please verify that Ollama is running and accessible at %s.\n", endpoint)
	}
}

// FormatLLMPipelineResult prints the LLM explanation, suggested script, and suggested command.
func FormatLLMPipelineResult(w io.Writer, exp *PipelineExplanation, opts FormatOptions) {
	if exp == nil || !exp.LLMQueried {
		return
	}

	if exp.LLMExplanation != "" {
		fmt.Fprintln(w)
		if opts.Color {
			theme := ui.GetTheme()
			fmt.Fprintf(w, "%sLLM Explanation:%s\n", theme.Info, theme.Reset)
		} else {
			fmt.Fprintln(w, "LLM Explanation:")
		}
		for _, line := range strings.Split(exp.LLMExplanation, "\n") {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	if exp.SuggestedScript != "" {
		FormatSuggestedScript(w, exp.SuggestedScript, opts)
	}

	if exp.SuggestedCommand != "" {
		FormatSuggestedCommand(w, exp.SuggestedCommand, opts)
	}
}

// FormatSuggestedScript prints the script block with line numbers, summarizing
// it with first 5 and last 5 lines if it exceeds 12 lines and (terminal lines - 10).
func FormatSuggestedScript(w io.Writer, script string, opts FormatOptions) {
	if script == "" {
		return
	}
	fmt.Fprintln(w)
	theme := ui.GetTheme()
	if opts.Color {
		fmt.Fprintf(w, "%sSuggested script:%s\n", theme.Bold, theme.Reset)
	} else {
		fmt.Fprintln(w, "Suggested script:")
	}

	rawLines := strings.Split(strings.TrimRight(script, "\r\n"), "\n")
	n := len(rawLines)
	termHeight := GetTerminalHeight()
	shouldTruncate := n > 12 && n > (termHeight-10)

	pad := len(fmt.Sprintf("%d", n))
	if pad < 2 {
		pad = 2
	}

	printLine := func(lineNum int, line string) {
		if opts.Color {
			fmt.Fprintf(w, "  %s%*d |%s %s\n", theme.Muted, pad, lineNum, theme.Reset, line)
		} else {
			fmt.Fprintf(w, "  %*d | %s\n", pad, lineNum, line)
		}
	}

	if shouldTruncate {
		for i := 0; i < 5; i++ {
			printLine(i+1, rawLines[i])
		}
		if opts.Color {
			fmt.Fprintf(w, "  %s-----------------------%s\n", theme.Muted, theme.Reset)
			fmt.Fprintf(w, "  %s(...)%s\n", theme.Muted, theme.Reset)
			fmt.Fprintf(w, "  %s-----------------------%s\n", theme.Muted, theme.Reset)
		} else {
			fmt.Fprintln(w, "  -----------------------")
			fmt.Fprintln(w, "  (...)")
			fmt.Fprintln(w, "  -----------------------")
		}
		for i := n - 5; i < n; i++ {
			printLine(i+1, rawLines[i])
		}
	} else {
		for i := 0; i < n; i++ {
			printLine(i+1, rawLines[i])
		}
	}
}

// FormatSuggestedCommand prints the suggested command block.
func FormatSuggestedCommand(w io.Writer, cmd string, opts FormatOptions) {
	if cmd == "" {
		return
	}
	fmt.Fprintln(w)
	if opts.Color {
		theme := ui.GetTheme()
		fmt.Fprintf(w, "%sSuggested command:%s\n  %s%s%s\n", theme.Bold, theme.Reset, theme.Success, cmd, theme.Reset)
	} else {
		fmt.Fprintf(w, "Suggested command:\n  %s\n", cmd)
	}
}

// FormatLLMResult prints the explanation, suggested command, and script for a single command.
func FormatLLMResult(w io.Writer, exp *CommandExplanation, opts FormatOptions) {
	if exp == nil {
		return
	}
	pExp := &PipelineExplanation{
		LLMExplanation:   exp.LLMExplanation,
		SuggestedCommand: exp.SuggestedCommand,
		SuggestedScript:  exp.SuggestedScript,
		LLMQueried:       exp.LLMQueried,
	}
	FormatLLMPipelineResult(w, pExp, opts)
}

// FormatPromptChoice renders the choice prompt in prompt color with underlined options.
func FormatPromptChoice(opts FormatOptions) string {
	if opts.Color {
		theme := ui.GetTheme()
		return theme.Prompt + "Do you want to run this command? [" +
			theme.Underline + "Y" + theme.Reset + theme.Prompt + "es/" +
			theme.Underline + "n" + theme.Reset + theme.Prompt + "o/" +
			theme.Underline + "e" + theme.Reset + theme.Prompt + "dit/" +
			theme.Underline + "m" + theme.Reset + theme.Prompt + "odifications] (default: Yes): " + theme.Reset
	}
	return "Do you want to run this command? [Yes/no/edit/modifications] (default: Yes): "
}

// FormatPromptChoiceScript renders the choice prompt for scripts (default: Edit).
func FormatPromptChoiceScript(opts FormatOptions) string {
	if opts.Color {
		theme := ui.GetTheme()
		return theme.Prompt + "Do you want to run this script? [" +
			theme.Underline + "y" + theme.Reset + theme.Prompt + "es/" +
			theme.Underline + "n" + theme.Reset + theme.Prompt + "o/" +
			theme.Underline + "E" + theme.Reset + theme.Prompt + "dit/" +
			theme.Underline + "m" + theme.Reset + theme.Prompt + "odifications] (default: Edit): " + theme.Reset
	}
	return "Do you want to run this script? [yes/no/Edit/modifications] (default: Edit): "
}

// FormatPromptChoiceNoMod renders the choice prompt without modifications option.
func FormatPromptChoiceNoMod(opts FormatOptions) string {
	if opts.Color {
		theme := ui.GetTheme()
		return theme.Prompt + "Do you want to run this command? [" +
			theme.Underline + "Y" + theme.Reset + theme.Prompt + "es/" +
			theme.Underline + "n" + theme.Reset + theme.Prompt + "o/" +
			theme.Underline + "e" + theme.Reset + theme.Prompt + "dit] (default: Yes): " + theme.Reset
	}
	return "Do you want to run this command? [Yes/no/edit] (default: Yes): "
}

func formatFlag(w io.Writer, flag FlagExplanation, opts FormatOptions) {
	flagName := flag.Flag.Name
	theme := ui.GetTheme()
	if flag.Found {
		if opts.Color {
			fmt.Fprintf(w, "%s%s found:%s\n", theme.Warning, flagName, theme.Reset)
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
			fmt.Fprintf(w, "%s%s%s: No description found.\n", theme.Error, flagName, theme.Reset)
		} else {
			fmt.Fprintf(w, "%s: No description found.\n", flagName)
		}
	}
}
