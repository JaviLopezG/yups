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

// Message represents a single chat turn for Ollama's /api/chat.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the payload sent to Ollama's /api/chat.
type ChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
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
		httpClient = &http.Client{Timeout: 15 * time.Second}
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
