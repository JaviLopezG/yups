package explain

import (
	"bytes"
	"regexp"
	"strings"
)

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// CleanManOutput removes backspaces (used for man bold/underline formatting
// like "c\bc" or "_\bc") and ANSI escape codes from manual text.
func CleanManOutput(raw string) string {
	var buf bytes.Buffer
	runes := []rune(raw)
	n := len(runes)

	for i := 0; i < n; i++ {
		if runes[i] == '\b' {
			// Backspace: remove preceding character from buffer
			// Actually in roff/groff formatting: 'X' '\b' 'X' (bold) or '_' '\b' 'X' (underline)
			// When we encounter '\b', the preceding char is in the buffer, and next char is runes[i+1].
			// If we just skip '\b' and the duplicate char, we get the clean char.
			continue
		}
		if i+1 < n && runes[i+1] == '\b' {
			// Skip this char because it's followed by backspace
			continue
		}
		buf.WriteRune(runes[i])
	}

	clean := buf.String()
	clean = ansiEscapeRegex.ReplaceAllString(clean, "")
	return clean
}

// ExtractManSummary extracts the command summary from the NAME or NOMBRE
// section of a manual page.
func ExtractManSummary(manContent string) string {
	clean := CleanManOutput(manContent)
	lines := strings.Split(clean, "\n")
	n := len(lines)

	inName := false
	for i := 0; i < n; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "NAME" || trimmed == "NOMBRE" {
			inName = true
			continue
		}
		if inName {
			if trimmed == "" {
				continue
			}
			// Check if we hit the next section (all uppercase word like SYNOPSIS, SINOPSIS, DESCRIPTION)
			if isManSectionHeader(trimmed) {
				break
			}
			return trimmed
		}
	}
	return ""
}

// FindOptionInMan searches for a flag (e.g. "-a", "--all", "-type") inside
// the manual page content. It returns the formatted option signature and
// description, or ("", false) if not found.
func FindOptionInMan(manContent, flag string) (string, bool) {
	if manContent == "" || flag == "" {
		return "", false
	}

	clean := CleanManOutput(manContent)
	cleanFlag := cleanFlagName(flag)
	if cleanFlag == "" {
		return "", false
	}

	lines := strings.Split(clean, "\n")
	n := len(lines)

	escaped := regexp.QuoteMeta(cleanFlag)
	pattern := regexp.MustCompile(`(?m)^\s*(?:` +
		escaped + `(?:[=,\[\s\)]|$)|` +
		`-[a-zA-Z0-9],\s*` + escaped + `(?:[=,\[\s\)]|$)|` +
		`--[a-zA-Z0-9_-]+,\s*` + escaped + `(?:[=,\[\s\)]|$)` +
		`)`)

	for i := 0; i < n; i++ {
		line := lines[i]
		if !pattern.MatchString(line) {
			continue
		}

		baseIndent := countLeadingSpaces(line)
		var collected []string
		collected = append(collected, strings.TrimRight(line, " \r\t"))

		// Collect following description lines until next option or section header
		for j := i + 1; j < n; j++ {
			nextLine := lines[j]
			trimmed := strings.TrimSpace(nextLine)
			if trimmed == "" {
				// Empty lines inside a description block are acceptable, but if followed
				// by something at the same or lesser indent, we stop.
				if j+1 < n {
					afterEmpty := lines[j+1]
					if isManSectionHeader(strings.TrimSpace(afterEmpty)) || (strings.TrimSpace(afterEmpty) != "" && countLeadingSpaces(afterEmpty) <= baseIndent) {
						break
					}
				}
				collected = append(collected, "")
				continue
			}

			if isManSectionHeader(trimmed) {
				break
			}

			indent := countLeadingSpaces(nextLine)
			if indent <= baseIndent && isOptionStart(nextLine) {
				break
			}

			collected = append(collected, strings.TrimRight(nextLine, " \r\t"))
		}

		// Trim trailing empty lines
		for len(collected) > 0 && strings.TrimSpace(collected[len(collected)-1]) == "" {
			collected = collected[:len(collected)-1]
		}

		if len(collected) > 0 {
			return strings.Join(collected, "\n"), true
		}
	}

	return "", false
}

func isManSectionHeader(line string) bool {
	if len(line) < 2 {
		return false
	}
	// Check if line is all uppercase e.g. "SYNOPSIS", "DESCRIPTION", "OPTIONS", "VEA TAMBIÉN"
	for _, r := range line {
		if (r >= 'a' && r <= 'z') || r == '-' {
			return false
		}
	}
	return true
}
