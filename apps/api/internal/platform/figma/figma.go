// Package figma imports component names from a Figma file.
package figma

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client reads Figma files.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New returns a client; an empty token disables it (nil).
func New(baseURL, token string) *Client {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = "https://api.figma.com"
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Component is one Figma COMPONENT node.
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type node struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Children []node `json:"children"`
}

// Components returns up to 200 COMPONENT nodes from a file.
func (c *Client) Components(ctx context.Context, fileKey string) ([]Component, error) {
	if c == nil {
		return nil, fmt.Errorf("figma: no token configured")
	}
	if strings.TrimSpace(fileKey) == "" {
		return nil, fmt.Errorf("figma: file key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/files/"+fileKey, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Figma-Token", c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("figma: request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("figma: status %d", res.StatusCode)
	}
	var payload struct {
		Document node `json:"document"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("figma: decode: %w", err)
	}
	var out []Component
	var walk func(n node)
	walk = func(n node) {
		if len(out) >= 200 {
			return
		}
		if n.Type == "COMPONENT" {
			out = append(out, Component{ID: n.ID, Name: n.Name})
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(payload.Document)
	return out, nil
}
