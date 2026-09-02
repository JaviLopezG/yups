package llm

import (
	"strings"
	"time"
)

// HistoryEntry represents a single shell history command with optional timestamp.
type HistoryEntry struct {
	Timestamp string
	Command   string
}

// LLMEnv defines the environment operations needed to collect system context.
type LLMEnv interface {
	UserHomeDir() (string, error)
	Getwd() (string, error)
	ReadOSRelease() string
	ReadHistory(home string, maxLines int) []HistoryEntry
	ReadFileSnippet(path string, maxLines int) (string, error)
	ListDirNames(dir string, maxItems int) []string
}

// SystemContext holds environment metadata gathered to assist the LLM.
type SystemContext struct {
	OSRelease     string
	CWD           string
	CWDListing    []string
	ParentListing []string
	FileSnippets  map[string]string // filepath -> text snippet
	RecentHistory []HistoryEntry
	CurrentTime   string
}

// CheatsheetDoc represents a single cheatsheet snippet.
type CheatsheetDoc struct {
	Source  string
	Name    string
	Content string
}

// CommandDoc holds full help, man page, and cheatsheets for an identified command.
type CommandDoc struct {
	Command     string
	Subcommand  string
	HelpOutput  string
	ManOutput   string
	Cheatsheets []CheatsheetDoc
}

// GatherContext collects low-cost system context (distro, cwd, directory files,
// referenced file snippets, current time, and recent shell history) to give the LLM situational awareness.
func GatherContext(env LLMEnv, referencedFiles []string) SystemContext {
	ctx := SystemContext{
		FileSnippets: make(map[string]string),
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05"),
	}
	if env == nil {
		return ctx
	}

	// 1. OS Release
	if osRel := env.ReadOSRelease(); osRel != "" {
		ctx.OSRelease = osRel
	} else {
		ctx.OSRelease = "Linux"
	}

	// 2. Working directory and directory listings
	if cwd, err := env.Getwd(); err == nil && cwd != "" {
		ctx.CWD = cwd
	}
	if names := env.ListDirNames(".", 20); len(names) > 0 {
		ctx.CWDListing = names
	}
	if parentNames := env.ListDirNames("..", 10); len(parentNames) > 0 {
		ctx.ParentListing = parentNames
	}

	// 3. File snippets for referenced files (limit to at most 3 files)
	seen := make(map[string]bool)
	count := 0
	for _, f := range referencedFiles {
		clean := strings.TrimSpace(f)
		if clean == "" || seen[clean] || strings.HasPrefix(clean, "-") {
			continue
		}
		seen[clean] = true
		if snippet, err := env.ReadFileSnippet(clean, 20); err == nil && snippet != "" {
			ctx.FileSnippets[clean] = snippet
			count++
			if count >= 3 {
				break
			}
		}
	}

	// 4. Shell history
	if home, err := env.UserHomeDir(); err == nil && home != "" {
		if history := env.ReadHistory(home, 5); len(history) > 0 {
			ctx.RecentHistory = history
		}
	}

	return ctx
}
