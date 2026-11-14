package git

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitLab talks to the GitLab REST API with a project/group access token.
type GitLab struct {
	BaseURL       string
	Token         string
	Secret        string
	Client        *http.Client
	RebaseTimeout time.Duration
	RebasePoll    time.Duration
}

// NewGitLab returns a GitLab provider.
func NewGitLab(token, secret string) *GitLab {
	return &GitLab{BaseURL: "https://gitlab.com", Token: token, Secret: secret, Client: http.DefaultClient}
}

// Name implements Provider.
func (g *GitLab) Name() string { return "gitlab" }

// CreatePullRequest opens a merge request.
func (g *GitLab) CreatePullRequest(ctx context.Context, req CreateRequest) (PullRequest, error) {
	payload, _ := json.Marshal(map[string]string{
		"source_branch": req.Head,
		"target_branch": req.Base,
		"title":         req.Title,
		"description":   req.Body,
	})
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", g.baseURL(), url.PathEscape(req.Repo))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return PullRequest{}, err
	}
	httpReq.Header.Set("PRIVATE-TOKEN", g.Token)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := g.client().Do(httpReq)
	if err != nil {
		return PullRequest{}, fmt.Errorf("gitlab: create merge request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return PullRequest{}, fmt.Errorf("gitlab: create merge request: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return PullRequest{}, fmt.Errorf("gitlab: decode merge request: %w", err)
	}
	return PullRequest{Number: out.IID, URL: out.WebURL, State: out.State}, nil
}

// MergePullRequest merges a merge request.
func (g *GitLab) MergePullRequest(ctx context.Context, repo string, number int, commitTitle string) error {
	payload, _ := json.Marshal(map[string]string{"squash": "true", "merge_commit_message": commitTitle})
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/merge", g.baseURL(), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: merge request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return fmt.Errorf("gitlab: merge request: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ParseWebhook verifies X-Gitlab-Token and normalises the payload.
func (g *GitLab) ParseWebhook(r *http.Request, body []byte) (WebhookEvent, error) {
	if g.Secret != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(g.Secret)) != 1 {
			return WebhookEvent{}, errors.New("gitlab: invalid webhook token")
		}
	}

	if r.Header.Get("X-Gitlab-Event") == "Pipeline Hook" {
		var pipeline struct {
			ObjectAttributes struct {
				Status   string `json:"status"`
				Duration int    `json:"duration"`
			} `json:"object_attributes"`
			MergeRequest struct {
				IID int `json:"iid"`
			} `json:"merge_request"`
			Project struct {
				PathWithNamespace string `json:"path_with_namespace"`
			} `json:"project"`
		}
		if err := json.Unmarshal(body, &pipeline); err != nil {
			return WebhookEvent{}, fmt.Errorf("gitlab: decode pipeline: %w", err)
		}
		status := "pending"
		switch pipeline.ObjectAttributes.Status {
		case "success":
			status = "passed"
		case "failed", "canceled":
			status = "failed"
		case "running":
			status = "running"
		}
		return WebhookEvent{
			Kind: "check_run", Repo: pipeline.Project.PathWithNamespace,
			Number: pipeline.MergeRequest.IID, CheckName: "pipeline", CheckStatus: status,
			CheckDurationMS: pipeline.ObjectAttributes.Duration * 1000,
		}, nil
	}

	var payload struct {
		ObjectKind string `json:"object_kind"`
		Project    struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
		ObjectAttributes struct {
			IID    int    `json:"iid"`
			Action string `json:"action"`
			State  string `json:"state"`
		} `json:"object_attributes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, fmt.Errorf("gitlab: decode webhook: %w", err)
	}
	return WebhookEvent{
		Kind:   payload.ObjectKind,
		Repo:   payload.Project.PathWithNamespace,
		Number: payload.ObjectAttributes.IID,
		Action: payload.ObjectAttributes.Action,
		Merged: payload.ObjectAttributes.State == "merged",
	}, nil
}

func (g *GitLab) baseURL() string {
	if g.BaseURL == "" {
		return "https://gitlab.com"
	}
	return strings.TrimRight(g.BaseURL, "/")
}

func (g *GitLab) client() *http.Client {
	if g.Client == nil {
		return http.DefaultClient
	}
	return g.Client
}

// RebasePullRequest asks GitLab to rebase an MR and waits for the asynchronous
// result. GitLab returns 202 immediately; the conflict state appears on the MR.
func (g *GitLab) RebasePullRequest(ctx context.Context, repo string, number int) error {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/rebase", g.baseURL(), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: rebase MR #%d: %w", number, err)
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("gitlab: rebase MR #%d: status %d: %s", number, res.StatusCode, strings.TrimSpace(string(body)))
	}

	timeout := g.RebaseTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	poll := g.RebasePoll
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		mr, err := g.getMergeRequest(ctx, repo, number)
		if err != nil {
			return err
		}
		if mr.RebaseInProgress {
			if time.Now().After(deadline) {
				return fmt.Errorf("gitlab: rebase MR #%d: still in progress after %s", number, timeout)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(poll):
				continue
			}
		}
		if mr.HasConflicts || strings.HasPrefix(mr.MergeStatus, "cannot_be_merged") || mr.DetailedMergeStatus == "conflict" || mr.MergeError != "" {
			detail := mr.MergeError
			if detail == "" {
				detail = mr.MergeStatus
			}
			return fmt.Errorf("gitlab: rebase MR #%d: conflict: %s", number, detail)
		}
		return nil
	}
}

type gitlabMergeRequest struct {
	IID                 int    `json:"iid"`
	HasConflicts        bool   `json:"has_conflicts"`
	MergeStatus         string `json:"merge_status"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
	RebaseInProgress    bool   `json:"rebase_in_progress"`
	MergeError          string `json:"merge_error"`
}

func (g *GitLab) getMergeRequest(ctx context.Context, repo string, number int) (gitlabMergeRequest, error) {
	var out gitlabMergeRequest
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d", g.baseURL(), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	res, err := g.client().Do(req)
	if err != nil {
		return out, fmt.Errorf("gitlab: read MR #%d: %w", number, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return out, fmt.Errorf("gitlab: read MR #%d: status %d", number, res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return out, fmt.Errorf("gitlab: decode MR #%d: %w", number, err)
	}
	return out, nil
}

// ProtectBranch protects a branch: no direct pushes, maintainers merge.
func (g *GitLab) ProtectBranch(ctx context.Context, repo, branch string, _ []string) error {
	payload, _ := json.Marshal(map[string]any{
		"name": branch, "push_access_level": 0, "merge_access_level": 30, "allow_force_push": false,
	})
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/protected_branches", g.baseURL(), url.PathEscape(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: protect branch: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("gitlab: protect branch: status %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

// PullRequestDiff fetches changed paths/diffs for an MR and renders a unified
// diff header per file (enough for policy evaluation, capped at 5 MiB).
func (g *GitLab) PullRequestDiff(ctx context.Context, repo string, number int) (string, error) {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/changes", g.baseURL(), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Token)
	res, err := g.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab: mr diff: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("gitlab: mr diff: status %d", res.StatusCode)
	}
	var payload struct {
		Changes []struct {
			NewPath string `json:"new_path"`
			Diff    string `json:"diff"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 5<<20)).Decode(&payload); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, change := range payload.Changes {
		b.WriteString("--- a/" + change.NewPath + "\n+++ b/" + change.NewPath + "\n")
		b.WriteString(change.Diff + "\n")
	}
	return b.String(), nil
}
