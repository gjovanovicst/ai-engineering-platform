// Package kms wraps/unwraps data keys with an HTTP KMS (or KMS proxy).
package kms

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client calls POST /wrap and POST /unwrap with base64 payloads.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New returns nil when the URL is empty (local master key only).
func New(baseURL, token string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

type request struct {
	Plaintext  string `json:"plaintext,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty"`
	KeyID      string `json:"key_id,omitempty"`
}

type response struct {
	Ciphertext string `json:"ciphertext,omitempty"`
	Plaintext  string `json:"plaintext,omitempty"`
}

func (c *Client) do(ctx context.Context, path string, in request) (response, error) {
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return response{}, fmt.Errorf("kms: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return response{}, fmt.Errorf("kms: status %d", res.StatusCode)
	}
	var out response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return response{}, fmt.Errorf("kms: decode: %w", err)
	}
	return out, nil
}

// Wrap encrypts a data key with the KMS master key.
func (c *Client) Wrap(ctx context.Context, plain []byte) ([]byte, error) {
	return c.WrapKey(ctx, "", plain)
}

// WrapKey encrypts a data key under a specific KMS key id.
func (c *Client) WrapKey(ctx context.Context, keyID string, plain []byte) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("kms: not configured")
	}
	res, err := c.do(ctx, "/wrap", request{Plaintext: base64.StdEncoding.EncodeToString(plain), KeyID: keyID})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(res.Ciphertext)
}

// Unwrap decrypts a wrapped data key.
func (c *Client) Unwrap(ctx context.Context, wrapped []byte) ([]byte, error) {
	return c.UnwrapKey(ctx, "", wrapped)
}

// UnwrapKey decrypts a data key using a specific KMS key id.
func (c *Client) UnwrapKey(ctx context.Context, keyID string, wrapped []byte) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("kms: not configured")
	}
	res, err := c.do(ctx, "/unwrap", request{Ciphertext: base64.StdEncoding.EncodeToString(wrapped), KeyID: keyID})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(res.Plaintext)
}
