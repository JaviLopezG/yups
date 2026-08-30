package app

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"yups/internal/explain"
	"yups/internal/sessionlog"
)

// KnownFlags contains all standard user-facing command-line flags accepted by yups.
var KnownFlags = []string{
	"--help",
	"--version",
	"--install-yups",
	"--uninstall-yups",
	"--update-yups",
	"--advanced",
	"--model",
	"--query",
	"--test-models",
}

// LevenshteinDistance calculates the edit distance between two strings.
func LevenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n1, n2 := len(r1), len(r2)
	if n1 == 0 {
		return n2
	}
	if n2 == 0 {
		return n1
	}

	dp := make([][]int, n1+1)
	for i := range dp {
		dp[i] = make([]int, n2+1)
		dp[i][0] = i
	}
	for j := 0; j <= n2; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n1; i++ {
		for j := 1; j <= n2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			min := dp[i-1][j] + 1 // deletion
			if dp[i][j-1]+1 < min {
				min = dp[i][j-1] + 1 // insertion
			}
			if dp[i-1][j-1]+cost < min {
				min = dp[i-1][j-1] + cost // substitution
			}
			dp[i][j] = min
		}
	}
	return dp[n1][n2]
}

// FindFlagsByLetter returns all known flags whose name (without leading --) starts with the given letter, case-insensitively.
func FindFlagsByLetter(letter string, knownFlags []string) []string {
	var matches []string
	target := strings.ToLower(letter)
	for _, kf := range knownFlags {
		name := strings.ToLower(strings.TrimPrefix(kf, "--"))
		if strings.HasPrefix(name, target) {
			matches = append(matches, kf)
		}
	}
	sort.Strings(matches)
	return matches
}

// FindSimilarFlags returns known flags that start with unknown or have an edit distance <= 2.
func FindSimilarFlags(unknown string, knownFlags []string) []string {
	type match struct {
		flag     string
		dist     int
		isPrefix bool
	}

	var matches []match
	seen := make(map[string]bool)

	for _, kf := range knownFlags {
		if kf == unknown {
			continue
		}
		isPrefix := strings.HasPrefix(kf, unknown)
		dist := LevenshteinDistance(unknown, kf)

		if isPrefix || dist <= 2 {
			if !seen[kf] {
				seen[kf] = true
				matches = append(matches, match{
					flag:     kf,
					dist:     dist,
					isPrefix: isPrefix,
				})
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		// Prefixes take precedence
		if matches[i].isPrefix != matches[j].isPrefix {
			return matches[i].isPrefix
		}
		// Smaller distance first
		if matches[i].dist != matches[j].dist {
			return matches[i].dist < matches[j].dist
		}
		return matches[i].flag < matches[j].flag
	})

	var result []string
	for _, m := range matches {
		result = append(result, m.flag)
	}
	return result
}

// HandleUnknownFlag handles unrecognized CLI flags with fuzzy suggestions, short flag detection, and interactive execution.
func HandleUnknownFlag(env *Env, args []string, flagIdx int, stdout, stderr io.Writer, color bool, logger *sessionlog.SessionLogger) int {
	arg := args[flagIdx]
	flagPart := arg
	valPart := ""
	hasEqual := false
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		flagPart = parts[0]
		valPart = parts[1]
		hasEqual = true
	}

	explain.FormatInvocationHeader(stdout, strings.Join(args, " "), "", color)

	isShortFlag := strings.HasPrefix(flagPart, "-") && !strings.HasPrefix(flagPart, "--") && len(flagPart) == 2

	var matches []string
	var errNotice string

	if isShortFlag {
		letter := strings.TrimPrefix(flagPart, "-")
		matches = FindFlagsByLetter(letter, KnownFlags)
		if color {
			errNotice = fmt.Sprintf("yups: yups does not use short flags (%s%q%s)", "\x1b[1;31m", arg, "\x1b[0m")
		} else {
			errNotice = fmt.Sprintf("yups: yups does not use short flags (%q)", arg)
		}
	} else {
		matches = FindSimilarFlags(flagPart, KnownFlags)
		if color {
			errNotice = fmt.Sprintf("yups: unknown option %s%q%s", "\x1b[1;31m", arg, "\x1b[0m")
		} else {
			errNotice = fmt.Sprintf("yups: unknown option %q", arg)
		}
	}

	// Case 3: No similar flag found
	if len(matches) == 0 {
		fmt.Fprintf(stderr, "%s\n\n", errNotice)
		fmt.Fprint(stderr, helpText)
		if logger != nil {
			logger.LogIncident("CLI_USAGE_ERROR", "%s", errNotice)
			logger.LogConclusion("", "", "", "UNKNOWN_OPTION", ExitUsage)
		}
		return ExitUsage
	}

	// Case 1: Multiple similar flags found (> 1)
	if len(matches) > 1 {
		fmt.Fprintf(stderr, "%s\n\n", errNotice)
		fmt.Fprintln(stderr, "Did you mean one of these?")
		for _, m := range matches {
			fmt.Fprintf(stderr, "    %s\n", m)
		}
		fmt.Fprintln(stderr, "\nRun 'yups --help' for a list of available options.")
		if logger != nil {
			logger.LogIncident("CLI_USAGE_ERROR", "%s (suggested: %s)", errNotice, strings.Join(matches, ", "))
			logger.LogConclusion("", "", "", "UNKNOWN_OPTION_MULTIPLE_SUGGESTIONS", ExitUsage)
		}
		return ExitUsage
	}

	// Case 2: Exactly 1 similar flag found
	matchedFlag := matches[0]
	if hasEqual {
		matchedFlag = matchedFlag + "=" + valPart
	}

	newArgs := make([]string, len(args))
	copy(newArgs, args)
	newArgs[flagIdx] = matchedFlag

	suggestedCmd := ProgramName + " " + strings.Join(newArgs, " ")

	if isShortFlag {
		if color {
			fmt.Fprintf(stdout, "\nyups: yups does not use short flags (%s%q%s). Did you mean %s%q%s?\n", "\x1b[1;31m", arg, "\x1b[0m", "\x1b[1;32m", matchedFlag, "\x1b[0m")
		} else {
			fmt.Fprintf(stdout, "\nyups: yups does not use short flags (%q). Did you mean %q?\n", arg, matchedFlag)
		}
	} else {
		if color {
			fmt.Fprintf(stdout, "\nyups: unknown option %s%q%s. Did you mean %s%q%s?\n", "\x1b[1;31m", arg, "\x1b[0m", "\x1b[1;32m", matchedFlag, "\x1b[0m")
		} else {
			fmt.Fprintf(stdout, "\nyups: unknown option %q. Did you mean %q?\n", arg, matchedFlag)
		}
	}

	explain.FormatSuggestedCommand(stdout, suggestedCmd, explain.FormatOptions{Color: color})
	fmt.Fprintln(stdout)

	for {
		promptStr := explain.FormatPromptChoiceNoMod(explain.FormatOptions{Color: color})
		var choice string
		if env != nil && env.AskPrompt != nil {
			choice = strings.ToLower(strings.TrimSpace(env.AskPrompt(promptStr, "y")))
		} else {
			choice = "y"
		}

		switch choice {
		case "y", "yes", "":
			if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
				_ = os.WriteFile(marker, []byte("executed\n"), 0o600)
			}
			fmt.Fprintf(stdout, "\nExecuting: %s\n", suggestedCmd)
			if logger != nil {
				logger.LogSection("EXECUTE_SUGGESTED_FLAG")
			}
			if env != nil && env.ExecShell != nil {
				return env.ExecShell(suggestedCmd, stdout, stderr)
			}
			return Dispatch(env, newArgs, stdout, stderr)
		case "n", "no":
			fmt.Fprintln(stdout)
			fmt.Fprint(stdout, helpText)
			if logger != nil {
				logger.LogConclusion("", "", "", "SUGGESTION_DECLINED", ExitOK)
			}
			return ExitOK
		case "e", "edit":
			editPrompt := "Edit command"
			if color {
				editPrompt = "\x1b[38;5;214mEdit command\x1b[0m"
			}
			var edited string
			if env != nil && env.AskEditPrompt != nil {
				edited = env.AskEditPrompt(editPrompt, suggestedCmd)
			} else if env != nil && env.AskPrompt != nil {
				edited = env.AskPrompt(editPrompt, suggestedCmd)
			}
			edited = strings.TrimSpace(edited)
			if edited == "" {
				fmt.Fprintln(stdout)
				fmt.Fprint(stdout, helpText)
				return ExitOK
			}
			if marker := os.Getenv("YUPS_READLINE_MARKER"); marker != "" {
				_ = os.WriteFile(marker, []byte("executed\n"), 0o600)
			}
			fmt.Fprintf(stdout, "\nExecuting: %s\n", edited)
			if env != nil && env.ExecShell != nil {
				return env.ExecShell(edited, stdout, stderr)
			}
			return ExitOK
		default:
			fmt.Fprintln(stdout, "Please answer y (yes), n (no), or e (edit).")
		}
	}
}
