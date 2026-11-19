// Package llm is the direct Messages API client the assistant talks to (the
// agent runs use the sandbox backend in internal/agent; the assistant needs a
// plain tool loop without a sandbox).
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

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/assistant"
)

// Anthropic is the Messages API client.
type Anthropic struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// NewAnthropic wires the client; an empty base URL uses the public API.
func NewAnthropic(baseURL, apiKey, model string) *Anthropic {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Anthropic{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTP: &http.Client{Timeout: 90 * time.Second}}
}

type apiMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type apiTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`
}

type apiResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat sends the conversation and returns the text plus the tool calls.
func (c *Anthropic) Chat(ctx context.Context, system string, messages []assistant.Message, tools []assistant.ToolSpec) (assistant.Reply, error) {
	converted := make([]apiMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" {
			converted = append(converted, apiMessage{Role: "user", Content: []map[string]any{{"type": "text", "text": m.Content}}})
			continue
		}
		role := m.Role
		if role != "assistant" {
			role = "user"
		}
		converted = append(converted, apiMessage{Role: role, Content: []map[string]any{{"type": "text", "text": m.Content}}})
	}
	specs := make([]apiTool, 0, len(tools))
	for _, t := range tools {
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		specs = append(specs, apiTool{Name: t.Name, Description: t.Description, InputSchema: schema})
	}
	payload, err := json.Marshal(apiRequest{Model: c.Model, MaxTokens: 2048, System: system, Messages: converted, Tools: specs})
	if err != nil {
		return assistant.Reply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return assistant.Reply{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return assistant.Reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return assistant.Reply{}, err
	}
	var parsed apiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return assistant.Reply{}, fmt.Errorf("llm: invalid response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		message := "request failed"
		if parsed.Error != nil {
			message = parsed.Error.Message
		}
		return assistant.Reply{}, fmt.Errorf("llm: %d: %s", resp.StatusCode, message)
	}
	var reply assistant.Reply
	for _, part := range parsed.Content {
		switch part.Type {
		case "text":
			reply.Text += part.Text
		case "tool_use":
			reply.ToolCalls = append(reply.ToolCalls, assistant.ToolCall{ID: part.ID, Name: part.Name, Input: part.Input})
		}
	}
	return reply, nil
}
