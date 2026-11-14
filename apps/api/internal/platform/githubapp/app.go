// Package githubapp mints GitHub App installation tokens.
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Client exchanges a signed app JWT for installation access tokens and caches
// them until shortly before they expire.
type Client struct {
	AppID          int64
	InstallationID int64
	BaseURL        string
	HTTP           *http.Client

	key    *rsa.PrivateKey
	mu     sync.Mutex
	token  string
	expiry time.Time
}

// New loads the app private key (PKCS#1 or PKCS#8 PEM).
func New(appID, installationID int64, keyPath, baseURL string) (*Client, error) {
	raw, err := os.ReadFile(keyPath) //nolint:gosec // operator-provided key path
	if err != nil {
		return nil, fmt.Errorf("githubapp: read key: %w", err)
	}
	return NewFromPEM(appID, installationID, raw, baseURL)
}

// NewFromPEM builds a client from an in-memory private key (per-org app).
func NewFromPEM(appID, installationID int64, raw []byte, baseURL string) (*Client, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("githubapp: key is not PEM")
	}
	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := parsed.(*rsa.PrivateKey); ok {
			key = rsaKey
		}
	}
	if key == nil {
		return nil, fmt.Errorf("githubapp: unsupported private key")
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{AppID: appID, InstallationID: installationID, BaseURL: baseURL, key: key}, nil
}

// Token returns a valid installation token, refreshing it when needed.
func (c *Client) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiry.Add(-time.Minute)) {
		return c.token, nil
	}
	installation := c.InstallationID
	if installation == 0 {
		first, err := c.firstInstallation(ctx)
		if err != nil {
			return "", err
		}
		installation = first
	}
	jwt, err := c.signJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/app/installations/%d/access_tokens", c.BaseURL, installation), bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("githubapp: token request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("githubapp: token status %d: %s", res.StatusCode, string(raw))
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("githubapp: decode token: %w", err)
	}
	c.token, c.expiry = out.Token, out.ExpiresAt
	return c.token, nil
}

func (c *Client) firstInstallation(ctx context.Context) (int64, error) {
	jwt, err := c.signJWT()
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/app/installations", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := c.client().Do(req)
	if err != nil {
		return 0, fmt.Errorf("githubapp: installations: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return 0, fmt.Errorf("githubapp: installations status %d: %s", res.StatusCode, string(raw))
	}
	var list []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return 0, fmt.Errorf("githubapp: no installation found")
	}
	c.InstallationID = list[0].ID
	return list[0].ID, nil
}

func (c *Client) signJWT() (string, error) {
	now := time.Now()
	header := b64([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": strconv.FormatInt(c.AppID, 10),
	})
	payload := b64(claims)
	signing := header + "." + payload
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("githubapp: sign jwt: %w", err)
	}
	return signing + "." + b64(sig), nil
}

func b64(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}
