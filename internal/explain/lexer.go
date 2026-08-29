package explain

import (
	"strings"
	"unicode"
)

// TokenType represents the classification of a shell token.
type TokenType int

const (
	TokenWord         TokenType = iota
	TokenOpOr                   // ||
	TokenOpAnd                  // &&
	TokenOpPipeStderr           // |&
	TokenOpPipe                 // |
	TokenOpSemi                 // ;
	TokenOpBackground           // &
	TokenRedir                  // >, >>, <, 2>, 2>&1, &>, etc.
	TokenComment                // # comment text
)

// Token holds a single lexed shell token with its type.
type Token struct {
	Type  TokenType
	Value string
}

// Tokenize splits a shell command line or list of arguments into normalized
// tokens, recognizing quotes, escapes, shell operators, and natural language queries.
func Tokenize(args []string) []Token {
	if len(args) == 0 {
		return nil
	}

	// If a single argument is passed and contains spaces or shell operators,
	// parse it as a shell line.
	raw := joinArgs(args)
	trimmed := strings.TrimSpace(raw)
	if isNaturalLanguageQuery(trimmed) && !strings.HasPrefix(trimmed, "#") {
		return []Token{
			{Type: TokenComment, Value: trimmed},
		}
	}

	return tokenizeString(raw)
}

func isNaturalLanguageQuery(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "¿") || strings.HasPrefix(trimmed, "?") {
		return true
	}
	lower := strings.ToLower(trimmed)
	prefixes := []string{
		"cómo ", "como ", "how ", "how to ", "what ", "what is ", "where ",
		"why ", "quién ", "quien ", "cual ", "cuál ", "dónde ", "donde ",
		"ayuda ", "help ", "explicar ", "explain ", "muéstrame ", "show me ",
		"dime ", "tell me ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// joinArgs intelligently rebuilds the raw command line while preserving quotes
// when necessary and keeping raw comments untouched.
func joinArgs(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	var sb strings.Builder
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if i > 0 {
			sb.WriteByte(' ')
		}
		if strings.HasPrefix(arg, "#") {
			sb.WriteString(strings.Join(args[i:], " "))
			break
		}
		if needsQuotes(arg) {
			sb.WriteString(quoteArg(arg))
		} else {
			sb.WriteString(arg)
		}
	}
	return sb.String()
}

func needsQuotes(s string) bool {
	if s == "" {
		return true
	}
	// Control operators and redirects shouldn't be quoted when re-joining
	switch s {
	case "||", "&&", "|", "|&", ";", "&", ">", ">>", "<", "2>", "2>&1", "&>":
		return false
	}
	// If the string itself contains shell operators or comments, it is a sub-pipeline and must not be quoted
	if strings.ContainsAny(s, "|&;><#") {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '\'' || r == '\\' || r == '$' || r == '`' {
			return true
		}
	}
	return false
}

func quoteArg(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	// Double quote with escaping
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' || r == '$' || r == '`' {
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	sb.WriteByte('"')
	return sb.String()
}

func tokenizeString(input string) []Token {
	var tokens []Token
	runes := []rune(strings.TrimSpace(input))
	n := len(runes)
	i := 0

	for i < n {
		// Skip whitespace
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}

		// Check multi-character operators
		if i+1 < n {
			two := string(runes[i : i+2])
			switch two {
			case "||":
				tokens = append(tokens, Token{Type: TokenOpOr, Value: "||"})
				i += 2
				continue
			case "&&":
				tokens = append(tokens, Token{Type: TokenOpAnd, Value: "&&"})
				i += 2
				continue
			case "|&":
				tokens = append(tokens, Token{Type: TokenOpPipeStderr, Value: "|&"})
				i += 2
				continue
			case ">>":
				tokens = append(tokens, Token{Type: TokenRedir, Value: ">>"})
				i += 2
				continue
			case "&>":
				tokens = append(tokens, Token{Type: TokenRedir, Value: "&>"})
				i += 2
				continue
			case "2>":
				// Check for 2>&1
				if i+3 < n && string(runes[i:i+4]) == "2>&1" {
					tokens = append(tokens, Token{Type: TokenRedir, Value: "2>&1"})
					i += 4
					continue
				}
				tokens = append(tokens, Token{Type: TokenRedir, Value: "2>"})
				i += 2
				continue
			}
		}

		// Check comment
		if runes[i] == '#' {
			comment := strings.TrimSpace(string(runes[i+1:]))
			tokens = append(tokens, Token{Type: TokenComment, Value: comment})
			break
		}

		// Check single-character operators
		switch runes[i] {
		case '|':
			tokens = append(tokens, Token{Type: TokenOpPipe, Value: "|"})
			i++
			continue
		case ';':
			tokens = append(tokens, Token{Type: TokenOpSemi, Value: ";"})
			i++
			continue
		case '&':
			tokens = append(tokens, Token{Type: TokenOpBackground, Value: "&"})
			i++
			continue
		case '>', '<':
			tokens = append(tokens, Token{Type: TokenRedir, Value: string(runes[i])})
			i++
			continue
		}

		// Word parsing (handles quotes and escapes)
		var word strings.Builder
		for i < n && !unicode.IsSpace(runes[i]) {
			r := runes[i]

			// If we hit a comment, emit current word and the comment token
			if r == '#' {
				if word.Len() > 0 {
					tokens = append(tokens, Token{Type: TokenWord, Value: word.String()})
				}
				comment := strings.TrimSpace(string(runes[i+1:]))
				tokens = append(tokens, Token{Type: TokenComment, Value: comment})
				return tokens
			}

			// If we hit an unquoted operator, break the word unless it's inside quotes
			if (r == '|' || r == '&' || r == ';' || r == '>' || r == '<') && word.Len() > 0 {
				break
			}

			switch r {
			case '\\':
				if i+1 < n {
					i++
					word.WriteRune(runes[i])
				}
				i++
			case '\'':
				// Single-quoted literal (no escape inside)
				i++
				for i < n && runes[i] != '\'' {
					word.WriteRune(runes[i])
					i++
				}
				if i < n && runes[i] == '\'' {
					i++
				}
			case '"':
				// Double-quoted string (handles \" and \\)
				i++
				for i < n && runes[i] != '"' {
					if runes[i] == '\\' && i+1 < n && (runes[i+1] == '"' || runes[i+1] == '\\' || runes[i+1] == '$' || runes[i+1] == '`') {
						i++
						word.WriteRune(runes[i])
					} else {
						word.WriteRune(runes[i])
					}
					i++
				}
				if i < n && runes[i] == '"' {
					i++
				}
			default:
				word.WriteRune(r)
				i++
			}
		}

		if word.Len() > 0 {
			tokens = append(tokens, Token{Type: TokenWord, Value: word.String()})
		}
	}

	return tokens
}
