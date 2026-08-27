package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ModelInfo represents metadata of a local model reported by Ollama.
type ModelInfo struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	Size      int64  `json:"size"`
	Digest    string `json:"digest"`
	Family    string `json:"-"`
	Parameter string `json:"-"`
}

// tagsResponse matches Ollama's /api/tags JSON output.
type tagsResponse struct {
	Models []struct {
		Name    string `json:"name"`
		Model   string `json:"model"`
		Size    int64  `json:"size"`
		Digest  string `json:"digest"`
		Details struct {
			Family        string `json:"family"`
			ParameterSize string `json:"parameter_size"`
		} `json:"details"`
	} `json:"models"`
}

// ToolCall represents a tool invocation request made by the model.
type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the name and arguments of a tool call.
type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Tool defines an available function tool according to Ollama / OpenAI tool schema.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines the metadata and parameters of a tool.
type ToolFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  ToolParams `json:"parameters"`
}

// ToolParams defines the JSON schema parameters for a tool.
type ToolParams struct {
	Type       string              `json:"type"`
	Properties map[string]ToolProp `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// ToolProp defines a single property in a tool's parameters schema.
type ToolProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// DefaultTools returns the standard tools available to the LLM.
func DefaultTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "fetch_command_documentation",
				Description: "Fetch detailed local manual page, --help output, and community cheatsheets (tldr-pages, navi, cheat.sh, cheat) for a specific command and optional subcommand.",
				Parameters: ToolParams{
					Type: "object",
					Properties: map[string]ToolProp{
						"command": {
							Type:        "string",
							Description: "The command name to look up (e.g. 'ls', 'tar', 'git', 'ip')",
						},
						"subcommand": {
							Type:        "string",
							Description: "Optional subcommand to look up (e.g. 'commit', 'checkout')",
						},
					},
					Required: []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "command-run",
				Description: "You can ask for whitelisted commands execution. You can use any Bash way to combine them (&&, ||, |, ;, &). Avoid harmful commands.",
				Parameters: ToolParams{
					Type: "object",
					Properties: map[string]ToolProp{
						"command": {
							Type:        "string",
							Description: "The shell command to run (must only use whitelisted commands: ls, pwd, stat, file, du, df, find, locate, tree, cat, head, tail, grep, ps, free, uptime, lscpu, ip, ss, ping, dig, nslookup, etc.).",
						},
					},
					Required: []string{"command"},
				},
			},
		},
	}
}

// Message represents a single chat turn for Ollama's /api/chat.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest is the payload sent to Ollama's /api/chat.
type ChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Tools    []Tool         `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

// ChatResponse is the response received from Ollama's /api/chat.
type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

// Client connects directly to Ollama's HTTP API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient initializes an Ollama HTTP client for baseURL (e.g. "http://localhost:11434").
func NewClient(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	cleanURL := strings.TrimRight(baseURL, "/")
	if cleanURL == "" {
		cleanURL = "http://localhost:11434"
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    cleanURL,
	}
}

// BaseURL returns the configured base endpoint URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// ListModels queries GET /api/tags to discover installed models.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := c.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying Ollama tags from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("Ollama returned status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	var data tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding Ollama tags response: %w", err)
	}

	models := make([]ModelInfo, 0, len(data.Models))
	for _, m := range data.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		models = append(models, ModelInfo{
			Name:      name,
			Model:     m.Model,
			Size:      m.Size,
			Digest:    m.Digest,
			Family:    m.Details.Family,
			Parameter: m.Details.ParameterSize,
		})
	}

	return models, nil
}

// Chat queries POST /api/chat with a prompt and returns the generated message.
func (c *Client) Chat(ctx context.Context, chatReq ChatRequest) (ChatResponse, error) {
	url := c.baseURL + "/api/chat"
	chatReq.Stream = false
	if chatReq.Options == nil {
		chatReq.Options = map[string]any{"temperature": 0.1}
	}

	payload, err := json.Marshal(chatReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("encoding chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("creating chat request for %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("sending chat request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return ChatResponse{}, fmt.Errorf("Ollama chat returned status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return ChatResponse{}, fmt.Errorf("decoding Ollama chat response: %w", err)
	}

	return chatResp, nil
}
