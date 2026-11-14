package git

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Gitea talks to the Gitea/Forgejo REST API (v1).
type Gitea struct {
	Token   string
	Secret  []byte
	BaseURL string
	HTTP    *http.Client
}

// NewGitea returns an adapter; baseURL defaults to gitea.com.
func NewGitea(token string, secret []byte) *Gitea {
	return &Gitea{Token: token, Secret: secret, BaseURL: "https://gitea.com/api/v1"}
}

// Name implements Provider.
func (g *Gitea) Name() string { return "gitea" }

func (g *Gitea) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (g *Gitea) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(g.BaseURL, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("gitea: %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("gitea: %s %s: status %d: %s", method, path, res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("gitea: decode: %w", err)
		}
	}
	return nil
}

// CreatePullRequest opens a pull request.
func (g *Gitea) CreatePullRequest(ctx context.Context, req CreateRequest) (PullRequest, error) {
	var out struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
	}
	err := g.do(ctx, http.MethodPost, "/repos/"+req.Repo+"/pulls", map[string]any{
		"title": req.Title, "body": req.Body, "head": req.Head, "base": req.Base,
	}, &out)
	if err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: out.Number, URL: out.HTMLURL, State: out.State}, nil
}

// MergePullRequest merges a pull request.
func (g *Gitea) MergePullRequest(ctx context.Context, repo string, number int, _ string) error {
	return g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, number), map[string]any{"Do": "merge"}, nil)
}

// ParseWebhook verifies X-Gitea-Signature and normalises the payload.
func (g *Gitea) ParseWebhook(r *http.Request, body []byte) (WebhookEvent, error) {
	if len(g.Secret) > 0 {
		signature := r.Header.Get("X-Gitea-Signature")
		mac := hmac.New(sha256.New, g.Secret)
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(want)) {
			return WebhookEvent{}, errors.New("gitea: invalid webhook signature")
		}
	}
	if r.Header.Get("X-Gitea-Event") != "pull_request" {
		return WebhookEvent{}, nil
	}
	var payload struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Merged  bool   `json:"merged"`
			HTMLURL string `json:"html_url"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, fmt.Errorf("gitea: decode webhook: %w", err)
	}
	return WebhookEvent{
		Kind: "merge_request", Repo: payload.Repository.FullName, Number: payload.Number,
		Action: payload.Action, Merged: payload.Action == "closed" && payload.PullRequest.Merged,
	}, nil
}
