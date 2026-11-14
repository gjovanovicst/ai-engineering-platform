// Package embeddings calls an OpenAI-compatible /embeddings endpoint.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal embeddings client.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// New returns a client; an empty URL means "not configured" (nil).
func New(baseURL, apiKey, model string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Embed returns one vector per input.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if c == nil {
		return nil, fmt.Errorf("embeddings: not configured")
	}
	body, _ := json.Marshal(map[string]any{"model": c.Model, "input": inputs})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings: request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings: status %d", res.StatusCode)
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embeddings: decode: %w", err)
	}
	if len(out.Data) != len(inputs) {
		return nil, fmt.Errorf("embeddings: expected %d vectors, got %d", len(inputs), len(out.Data))
	}
	vectors := make([][]float32, 0, len(out.Data))
	for _, item := range out.Data {
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
