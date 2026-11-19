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

// OpenAI is an OpenAI-compatible chat-completions client (DeepSeek, Ollama,
// vLLM, any gateway that speaks the same schema).
type OpenAI struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// NewOpenAI wires the client; an empty base URL uses DeepSeek's API.
func NewOpenAI(baseURL, apiKey, model string) *OpenAI {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	return &OpenAI{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTP: &http.Client{Timeout: 90 * time.Second}}
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type oaRequest struct {
	Model    string      `json:"model"`
	Messages []oaMessage `json:"messages"`
	Tools    []oaTool    `json:"tools,omitempty"`
}

type oaResponse struct {
	Choices []struct {
		Message struct {
			Content   string       `json:"content"`
			ToolCalls []oaToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat sends the conversation and returns the text plus the tool calls.
func (c *OpenAI) Chat(ctx context.Context, system string, messages []assistant.Message, tools []assistant.ToolSpec) (assistant.Reply, error) {
	converted := []oaMessage{{Role: "system", Content: system}}
	// Tool results map to the tool_call_id of the previous call; the assistant
	// appends them in order, so the ids are tracked here.
	for _, m := range messages {
		switch m.Role {
		case "tool":
			// The stored history keeps the tool output as a plain message (the
			// assistant message does not carry the tool_calls ids), so it is
			// replayed as a user turn: the OpenAI API rejects a tool role
			// without the matching tool_calls.
			converted = append(converted, oaMessage{Role: "user", Content: m.Content})
		case "assistant":
			converted = append(converted, oaMessage{Role: "assistant", Content: m.Content})
		default:
			converted = append(converted, oaMessage{Role: "user", Content: m.Content})
		}
	}
	specs := make([]oaTool, 0, len(tools))
	for _, t := range tools {
		params := t.InputSchema
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var spec oaTool
		spec.Type = "function"
		spec.Function.Name = t.Name
		spec.Function.Description = t.Description
		spec.Function.Parameters = params
		specs = append(specs, spec)
	}
	payload, err := json.Marshal(oaRequest{Model: c.Model, Messages: converted, Tools: specs})
	if err != nil {
		return assistant.Reply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return assistant.Reply{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return assistant.Reply{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return assistant.Reply{}, err
	}
	var parsed oaResponse
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
	if len(parsed.Choices) == 0 {
		return assistant.Reply{}, nil
	}
	var reply assistant.Reply
	message := parsed.Choices[0].Message
	reply.Text = message.Content
	for _, call := range message.ToolCalls {
		reply.ToolCalls = append(reply.ToolCalls, assistant.ToolCall{ID: call.ID, Name: call.Function.Name, Input: json.RawMessage(call.Function.Arguments)})
	}
	return reply, nil
}
