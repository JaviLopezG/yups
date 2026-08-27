package explain

import (
	"regexp"
	"strings"
)

// FindOptionInHelp searches for a flag (e.g. "-a", "--all", "-type") inside
// the raw text output of `command --help`. It returns the formatted option
// signature and description, or ("", false) if not found.
func FindOptionInHelp(helpText, flag string) (string, bool) {
	if helpText == "" || flag == "" {
		return "", false
	}

	cleanFlag := cleanFlagName(flag)
	if cleanFlag == "" {
		return "", false
	}

	lines := strings.Split(helpText, "\n")
	n := len(lines)

	// Regexp to match the flag at the beginning of an option entry
	// Handles: " -a, --all", " --all", " -a ", " -a[", " -a=", " -a|", " --color[=WHEN]"
	escaped := regexp.QuoteMeta(cleanFlag)
	pattern := regexp.MustCompile(`(?m)^\s*(?:` +
		escaped + `(?:[=,\[\s|]|$)|` + // flag at start of option list
		`-[a-zA-Z0-9],\s*` + escaped + `(?:[=,\[\s|]|$)|` + // short flag before this long flag
		`--[a-zA-Z0-9_-]+,\s*` + escaped + `(?:[=,\[\s|]|$)` + // long flag before this flag
		`)`)

	for i := 0; i < n; i++ {
		line := lines[i]
		if !pattern.MatchString(line) {
			continue
		}

		// Found candidate option line. Gather this line and any continuation lines.
		var collected []string
		collected = append(collected, strings.TrimRight(line, " \r\t"))

		// Detect continuation lines: indented further than option name,
		// not starting with '-' (new option), not empty, not section headers.
		baseIndent := countLeadingSpaces(line)
		for j := i + 1; j < n; j++ {
			nextLine := lines[j]
			trimmed := strings.TrimSpace(nextLine)
			if trimmed == "" {
				break
			}
			// If next line starts with a new option (e.g. "  -b, --...") break
			if isOptionStart(nextLine) {
				break
			}
			// If next line is indented deeper than baseIndent, it's continuation
			if countLeadingSpaces(nextLine) > baseIndent {
				collected = append(collected, strings.TrimRight(nextLine, " \r\t"))
			} else {
				break
			}
		}

		return strings.Join(collected, "\n"), true
	}

	return "", false
}

// cleanFlagName extracts the base flag without values (e.g. "--color=auto" -> "--color").
func cleanFlagName(flag string) string {
	flag = strings.TrimSpace(flag)
	if idx := strings.Index(flag, "="); idx != -1 {
		flag = flag[:idx]
	}
	return flag
}

func countLeadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
		} else if r == '\t' {
			count += 8
		} else {
			break
		}
	}
	return count
}

func isOptionStart(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "-") && len(trimmed) > 1 && trimmed[1] != ' '
}
