package git

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Bitbucket talks to the Bitbucket Cloud REST API v2. Token is
// "username:app-password" (or "x-token-auth:<access-token>").
type Bitbucket struct {
	Token   string
	Secret  []byte
	BaseURL string
	HTTP    *http.Client
}

// NewBitbucket returns an adapter.
func NewBitbucket(token string, secret []byte) *Bitbucket {
	return &Bitbucket{Token: token, Secret: secret, BaseURL: "https://api.bitbucket.org/2.0"}
}

// Name implements Provider.
func (b *Bitbucket) Name() string { return "bitbucket" }

func (b *Bitbucket) client() *http.Client {
	if b.HTTP != nil {
		return b.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (b *Bitbucket) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(b.BaseURL, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(b.Token)))
	req.Header.Set("Content-Type", "application/json")
	res, err := b.client().Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket: %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("bitbucket: %s %s: status %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("bitbucket: decode: %w", err)
		}
	}
	return nil
}

// CreatePullRequest opens a pull request.
func (b *Bitbucket) CreatePullRequest(ctx context.Context, req CreateRequest) (PullRequest, error) {
	var out struct {
		ID    int    `json:"id"`
		State string `json:"state"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}
	err := b.do(ctx, http.MethodPost, "/repositories/"+req.Repo+"/pullrequests", map[string]any{
		"title": req.Title, "description": req.Body,
		"source":              map[string]any{"branch": map[string]any{"name": req.Head}},
		"destination":         map[string]any{"branch": map[string]any{"name": req.Base}},
		"close_source_branch": true,
	}, &out)
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: out.ID, URL: out.Links.HTML.Href, State: out.State}, nil
}

// MergePullRequest merges a pull request.
func (b *Bitbucket) MergePullRequest(ctx context.Context, repo string, number int, message string) error {
	return b.do(ctx, http.MethodPost, fmt.Sprintf("/repositories/%s/pullrequests/%d/merge", repo, number),
		map[string]any{"message": message, "close_source_branch": true}, nil)
}

// ParseWebhook verifies X-Hub-Signature and normalises pullrequest events.
func (b *Bitbucket) ParseWebhook(r *http.Request, body []byte) (WebhookEvent, error) {
	if len(b.Secret) > 0 {
		signature := strings.TrimPrefix(r.Header.Get("X-Hub-Signature"), "sha256=")
		mac := hmac.New(sha256.New, b.Secret)
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(want)) {
			return WebhookEvent{}, errors.New("bitbucket: invalid webhook signature")
		}
	}
	event := r.Header.Get("X-Event-Key")
	if !strings.HasPrefix(event, "pullrequest:") {
		return WebhookEvent{}, nil
	}
	var payload struct {
		PullRequest struct {
			ID    int    `json:"id"`
			State string `json:"state"`
		} `json:"pullrequest"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, fmt.Errorf("bitbucket: decode webhook: %w", err)
	}
	merged := payload.PullRequest.State == "merged" || event == "pullrequest:fulfilled"
	return WebhookEvent{
		Kind: "merge_request", Repo: payload.Repository.FullName, Number: payload.PullRequest.ID,
		Action: strings.TrimPrefix(event, "pullrequest:"), Merged: merged,
	}, nil
}
