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

// GitHub talks to the GitHub REST API. Token is a function so that the caller
// can supply a short-lived GitHub App installation token.
type GitHub struct {
	BaseURL string
	Token   func(ctx context.Context) (string, error)
	Secret  []byte
	Client  *http.Client
}

// NewGitHub returns a GitHub provider.
func NewGitHub(token string, secret string) *GitHub {
	return &GitHub{
		BaseURL: "https://api.github.com",
		Token:   func(context.Context) (string, error) { return token, nil },
		Secret:  []byte(secret),
		Client:  http.DefaultClient,
	}
}

// Name implements Provider.
func (g *GitHub) Name() string { return "github" }

// CreatePullRequest opens a pull request.
func (g *GitHub) CreatePullRequest(ctx context.Context, req CreateRequest) (PullRequest, error) {
	token, err := g.Token(ctx)
	if err != nil {
		return PullRequest{}, err
	}
	payload, _ := json.Marshal(map[string]string{
		"title": req.Title, "body": req.Body, "head": req.Head, "base": req.Base,
	})
	url := fmt.Sprintf("%s/repos/%s/pulls", g.baseURL(), req.Repo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return PullRequest{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := g.client().Do(httpReq)
	if err != nil {
		return PullRequest{}, fmt.Errorf("github: create pull request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return PullRequest{}, fmt.Errorf("github: create pull request: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return PullRequest{}, fmt.Errorf("github: decode pull request: %w", err)
	}
	return PullRequest{Number: out.Number, URL: out.HTMLURL, State: out.State}, nil
}

// MergePullRequest merges a PR (squash, matching the platform default).
func (g *GitHub) MergePullRequest(ctx context.Context, repo string, number int, commitTitle string) error {
	token, err := g.Token(ctx)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"merge_method": "squash", "commit_title": commitTitle})
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/merge", g.baseURL(), repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("github: merge pull request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return fmt.Errorf("github: merge pull request: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ParseWebhook verifies X-Hub-Signature-256 and normalises the payload.
func (g *GitHub) ParseWebhook(r *http.Request, body []byte) (WebhookEvent, error) {
	if len(g.Secret) > 0 {
		signature := r.Header.Get("X-Hub-Signature-256")
		mac := hmac.New(sha256.New, g.Secret)
		mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(want)) {
			return WebhookEvent{}, errors.New("github: invalid webhook signature")
		}
	}

	if r.Header.Get("X-GitHub-Event") == "check_run" {
		var check struct {
			CheckRun struct {
				Name       string `json:"name"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				HeadSHA    string `json:"head_sha"`
				Output     struct {
					Summary string `json:"summary"`
				} `json:"output"`
				DetailsURL   string `json:"details_url"`
				HTMLURL      string `json:"html_url"`
				PullRequests []struct {
					Number int `json:"number"`
				} `json:"pull_requests"`
			} `json:"check_run"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &check); err != nil {
			return WebhookEvent{}, fmt.Errorf("github: decode check_run: %w", err)
		}
		number := 0
		if len(check.CheckRun.PullRequests) > 0 {
			number = check.CheckRun.PullRequests[0].Number
		}
		return WebhookEvent{
			Kind: "check_run", Repo: check.Repository.FullName, Number: number,
			CheckName: check.CheckRun.Name, CheckStatus: mapConclusion(check.CheckRun.Status, check.CheckRun.Conclusion),
			CheckOutput: check.CheckRun.Output.Summary, CommitSHA: check.CheckRun.HeadSHA,
			CheckURL: firstNonEmpty(check.CheckRun.DetailsURL, check.CheckRun.HTMLURL),
		}, nil
	}

	var payload struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Merged bool   `json:"merged"`
			State  string `json:"state"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, fmt.Errorf("github: decode webhook: %w", err)
	}
	return WebhookEvent{
		Kind: "merge_request", Repo: payload.Repository.FullName, Number: payload.Number,
		Action: payload.Action, Merged: payload.PullRequest.Merged,
	}, nil
}

func (g *GitHub) baseURL() string {
	if g.BaseURL == "" {
		return "https://api.github.com"
	}
	return strings.TrimRight(g.BaseURL, "/")
}

func (g *GitHub) client() *http.Client {
	if g.Client == nil {
		return http.DefaultClient
	}
	return g.Client
}

// mapConclusion normalises a GitHub check_run status/conclusion.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapConclusion(status, conclusion string) string {
	switch conclusion {
	case "success":
		return "passed"
	case "failure", "timed_out", "cancelled", "action_required", "startup_failure":
		return "failed"
	}
	switch status {
	case "in_progress":
		return "running"
	case "queued":
		return "pending"
	case "completed":
		return "passed"
	}
	return "pending"
}

// ListChecks returns the check runs of a pull request. It is used as a fallback
// when webhooks are unavailable: the worker polls open PRs and records results.
func (g *GitHub) ListChecks(ctx context.Context, repo string, number int) ([]WebhookEvent, error) {
	token, err := g.Token(ctx)
	if err != nil {
		return nil, err
	}
	do := func(url string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		res, err := g.client().Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if res.StatusCode >= 300 {
			return nil, fmt.Errorf("github: checks: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
		}
		return body, nil
	}

	pullBody, err := do(fmt.Sprintf("%s/repos/%s/pulls/%d", g.baseURL(), repo, number))
	if err != nil {
		return nil, err
	}
	var pull struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(pullBody, &pull); err != nil {
		return nil, fmt.Errorf("github: decode pull request: %w", err)
	}
	if pull.Head.SHA == "" {
		return nil, errors.New("github: pull request has no head sha")
	}

	body, err := do(fmt.Sprintf("%s/repos/%s/commits/%s/check-runs", g.baseURL(), repo, pull.Head.SHA))
	if err != nil {
		// Fine-grained tokens often lack checks:read. Actions jobs expose the same
		// result under actions:read, which is enough for the merge gate.
		return g.listActionJobs(ctx, do, repo, pull.Head.SHA, number)
	}
	var payload struct {
		CheckRuns []struct {
			Name        string     `json:"name"`
			Status      string     `json:"status"`
			Conclusion  string     `json:"conclusion"`
			StartedAt   *time.Time `json:"started_at"`
			CompletedAt *time.Time `json:"completed_at"`
			Output      struct {
				Summary string `json:"summary"`
			} `json:"output"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("github: decode check runs: %w", err)
	}
	out := make([]WebhookEvent, 0, len(payload.CheckRuns))
	for _, run := range payload.CheckRuns {
		duration := 0
		if run.StartedAt != nil && run.CompletedAt != nil {
			duration = int(run.CompletedAt.Sub(*run.StartedAt).Milliseconds())
		}
		out = append(out, WebhookEvent{
			Kind: "check_run", Repo: repo, Number: number,
			CheckName: run.Name, CheckStatus: mapConclusion(run.Status, run.Conclusion),
			CheckOutput: run.Output.Summary, CheckDurationMS: duration, CommitSHA: pull.Head.SHA,
		})
	}
	return out, nil
}

// listActionJobs reads workflow jobs as a fallback for check runs.
func (g *GitHub) listActionJobs(ctx context.Context, do func(string) ([]byte, error), repo, sha string, number int) ([]WebhookEvent, error) {
	body, err := do(fmt.Sprintf("%s/repos/%s/actions/runs?head_sha=%s&per_page=20", g.baseURL(), repo, sha))
	if err != nil {
		return nil, err
	}
	var runs struct {
		WorkflowRuns []struct {
			ID int64 `json:"id"`
		} `json:"workflow_runs"`
	}
	if err := json.Unmarshal(body, &runs); err != nil {
		return nil, fmt.Errorf("github: decode workflow runs: %w", err)
	}
	byName := map[string]WebhookEvent{}
	for _, run := range runs.WorkflowRuns {
		jobBody, jerr := do(fmt.Sprintf("%s/repos/%s/actions/runs/%d/jobs", g.baseURL(), repo, run.ID))
		if jerr != nil {
			return nil, jerr
		}
		var jobs struct {
			Jobs []struct {
				Name        string     `json:"name"`
				Status      string     `json:"status"`
				Conclusion  string     `json:"conclusion"`
				StartedAt   *time.Time `json:"started_at"`
				CompletedAt *time.Time `json:"completed_at"`
				HTMLURL     string     `json:"html_url"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(jobBody, &jobs); err != nil {
			return nil, fmt.Errorf("github: decode workflow jobs: %w", err)
		}
		for _, job := range jobs.Jobs {
			duration := 0
			if job.StartedAt != nil && job.CompletedAt != nil {
				duration = int(job.CompletedAt.Sub(*job.StartedAt).Milliseconds())
			}
			byName[job.Name] = WebhookEvent{
				Kind: "check_run", Repo: repo, Number: number, CheckName: job.Name,
				CheckStatus: mapConclusion(job.Status, job.Conclusion), CheckDurationMS: duration, CommitSHA: sha,
				CheckURL: job.HTMLURL,
			}
		}
	}
	out := make([]WebhookEvent, 0, len(byName))
	for _, event := range byName {
		out = append(out, event)
	}
	return out, nil
}

// ProtectBranch enables required status checks and one approving review for a
// branch. It is idempotent (PUT replaces the protection settings).
func (g *GitHub) ProtectBranch(ctx context.Context, repo, branch string, checks []string) error {
	if repo == "" || branch == "" {
		return fmt.Errorf("github: repo and branch are required")
	}
	body := map[string]any{
		"required_status_checks": map[string]any{"strict": true, "contexts": checks},
		"enforce_admins":         false,
		"required_pull_request_reviews": map[string]any{
			"required_approving_review_count": 1,
		},
		"restrictions": nil,
	}
	raw, _ := json.Marshal(body)
	token, err := g.Token(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/branches/%s/protection", g.baseURL(), repo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("github: protect branch: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("github: protect branch: status %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

// RebasePullRequest asks GitHub to update the PR branch from its base. A
// conflict or a disabled update-branch returns an error.
func (g *GitHub) RebasePullRequest(ctx context.Context, repo string, number int) error {
	token, err := g.Token(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/update-branch", g.baseURL(), repo, number)
	// expected_head_sha is optional; sending an empty string makes GitHub answer
	// 422 "expected head sha didn't match current head ref".
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("github: rebase: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		message := strings.TrimSpace(string(data))
		// A branch that already contains the base is not an error for the queue.
		lower := strings.ToLower(message)
		if res.StatusCode == http.StatusUnprocessableEntity &&
			(strings.Contains(lower, "up to date") || strings.Contains(lower, "no new commits") || strings.Contains(lower, "no changes")) {
			return nil
		}
		return fmt.Errorf("github: rebase PR #%d: status %d: %s", number, res.StatusCode, message)
	}
	return nil
}

// FindOpenPullRequest returns an existing open PR for a head branch.
func (g *GitHub) FindOpenPullRequest(ctx context.Context, repo, head string) (int, string, bool, error) {
	token, err := g.Token(ctx)
	if err != nil {
		return 0, "", false, err
	}
	owner := repo
	if i := strings.Index(repo, "/"); i > 0 {
		owner = repo[:i]
	}
	url := fmt.Sprintf("%s/repos/%s/pulls?state=open&head=%s:%s", g.baseURL(), repo, owner, head)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := g.client().Do(req)
	if err != nil {
		return 0, "", false, fmt.Errorf("github: find PR: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return 0, "", false, fmt.Errorf("github: find PR: status %d", res.StatusCode)
	}
	var pulls []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pulls); err != nil {
		return 0, "", false, err
	}
	if len(pulls) == 0 {
		return 0, "", false, nil
	}
	return pulls[0].Number, pulls[0].HTMLURL, true, nil
}

// PullRequestDiff fetches the unified diff of a PR (capped at 5 MiB).
func (g *GitHub) PullRequestDiff(ctx context.Context, repo string, number int) (string, error) {
	token, err := g.Token(ctx)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", g.baseURL(), repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	res, err := g.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("github: pr diff: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("github: pr diff: status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 5<<20))
	return string(data), err
}
