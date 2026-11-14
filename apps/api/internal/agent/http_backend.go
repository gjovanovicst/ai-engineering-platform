package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// HTTPBackend calls an OpenAI-compatible /chat/completions endpoint. It streams
// when the provider supports it and retries transient failures with backoff.
type HTTPBackend struct {
	BaseURL     string
	APIKey      string
	Model       string
	Client      *http.Client
	MaxAttempts int
	// baseDelay is replaceable in tests.
	baseDelay time.Duration
}

// NewHTTPBackend builds the backend for a provider configuration.
func NewHTTPBackend(baseURL, apiKey, model string) *HTTPBackend {
	return &HTTPBackend{
		BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model,
		Client: http.DefaultClient, MaxAttempts: 3, baseDelay: 500 * time.Millisecond,
	}
}

// Name implements Backend.
func (b *HTTPBackend) Name() string { return "openai-compatible" }

// Run implements Backend.
func (b *HTTPBackend) Run(ctx context.Context, spec Spec, emit func(Event) error) (Result, error) {
	model := spec.Model
	if model == "" {
		model = b.Model
	}
	if b.BaseURL == "" || model == "" {
		return Result{}, fmt.Errorf("agent: provider base_url and model are required")
	}
	attempts := b.MaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		res, err := b.request(ctx, model, spec.Prompt)
		if err != nil {
			lastErr = err
			if !b.retry(ctx, attempt, attempts) {
				return Result{}, err
			}
			continue
		}
		if retryable(res.StatusCode) {
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
			_ = res.Body.Close()
			lastErr = fmt.Errorf("agent: provider status %d", res.StatusCode)
			if !b.retry(ctx, attempt, attempts) {
				return Result{}, lastErr
			}
			continue
		}
		if res.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			_ = res.Body.Close()
			return Result{}, fmt.Errorf("agent: provider status %d: %s", res.StatusCode, truncateText(string(body), 500))
		}
		return b.consume(res, emit)
	}
	return Result{}, lastErr
}

func (b *HTTPBackend) request(ctx context.Context, model, prompt string) (*http.Response, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":          model,
		"messages":       []map[string]string{{"role": "user", "content": prompt}},
		"stream":         true,
		"stream_options": map[string]bool{"include_usage": true},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	return b.client().Do(req)
}

// retry sleeps with exponential backoff and jitter; false means "give up".
func (b *HTTPBackend) retry(ctx context.Context, attempt, attempts int) bool {
	if attempt >= attempts {
		return false
	}
	delay := b.baseDelay * time.Duration(1<<uint(attempt-1))
	if b.baseDelay == 0 {
		delay = 500 * time.Millisecond
	}
	delay += time.Duration(rand.Int63n(int64(delay/2 + 1))) //nolint:gosec // jitter, not security
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func (b *HTTPBackend) consume(res *http.Response, emit func(Event) error) (Result, error) {
	defer func() { _ = res.Body.Close() }()
	if strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		return b.consumeStream(res.Body, emit)
	}
	return b.consumeJSON(res.Body, emit)
}

// consumeStream parses SSE chunks and emits complete lines as they arrive.
func (b *HTTPBackend) consumeStream(body io.Reader, emit func(Event) error) (Result, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var pending, model string
	var usage Result
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // providers sometimes send keep-alive frames
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			usage.InputTokens = chunk.Usage.PromptTokens
			usage.OutputTokens = chunk.Usage.CompletionTokens
		}
		for _, choice := range chunk.Choices {
			pending += choice.Delta.Content
			for {
				idx := strings.IndexByte(pending, '\n')
				if idx < 0 {
					break
				}
				if err := emit(Event{Type: EventText, Text: strings.TrimRight(pending[:idx], "\r")}); err != nil {
					return Result{}, err
				}
				pending = pending[idx+1:]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Result{}, fmt.Errorf("agent: stream read: %w", err)
	}
	if strings.TrimSpace(pending) != "" {
		if err := emit(Event{Type: EventText, Text: pending}); err != nil {
			return Result{}, err
		}
	}
	if err := emit(Event{Type: EventDone}); err != nil {
		return Result{}, err
	}
	usage.Provider = "openai-compatible"
	usage.Model = model
	if usage.Model == "" {
		usage.Model = b.Model
	}
	return usage, nil
}

func (b *HTTPBackend) consumeJSON(body io.Reader, emit func(Event) error) (Result, error) {
	raw, _ := io.ReadAll(io.LimitReader(body, 4<<20))
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Result{}, fmt.Errorf("agent: decode provider response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Result{}, fmt.Errorf("agent: provider returned no choices")
	}
	content := out.Choices[0].Message.Content
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		if err := emit(Event{Type: EventText, Text: line}); err != nil {
			return Result{}, err
		}
	}
	if err := emit(Event{Type: EventDone}); err != nil {
		return Result{}, err
	}
	return Result{
		InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens,
		Provider: "openai-compatible", Model: b.Model,
	}, nil
}

func (b *HTTPBackend) client() *http.Client {
	if b.Client == nil {
		return http.DefaultClient
	}
	return b.Client
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
