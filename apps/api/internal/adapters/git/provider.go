// Package git abstracts the source host (GitHub, GitLab). Provider-specific
// auth, ids and webhook formats stay inside the adapters (ADR-0007).
package git

import (
	"context"
	"net/http"
)

// PullRequest is a normalised pull/merge request.
type PullRequest struct {
	Number int
	URL    string
	State  string
}

// CreateRequest describes a pull/merge request to open.
type CreateRequest struct {
	Repo  string // "owner/name" (GitHub) or "group/project" (GitLab)
	Title string
	Body  string
	Head  string
	Base  string
}

// WebhookEvent is the subset of a provider webhook the platform cares about.
type WebhookEvent struct {
	Kind   string // "merge_request" or "check_run"
	Repo   string
	Number int
	Action string // opened, closed, merged, ...
	Merged bool

	// Check results (Kind == "check_run").
	CheckName       string
	CheckStatus     string // passed, failed, running, pending
	CheckOutput     string
	CheckDurationMS int
	CommitSHA       string
}

// Provider is implemented by github.Provider and gitlab.Provider.
type Provider interface {
	Name() string
	CreatePullRequest(ctx context.Context, req CreateRequest) (PullRequest, error)
	// MergePullRequest merges an approved PR/MR.
	MergePullRequest(ctx context.Context, repo string, number int, commitTitle string) error
	// ParseWebhook verifies the signature/token and normalises the payload.
	ParseWebhook(r *http.Request, body []byte) (WebhookEvent, error)
}

// DefaultProviders builds the real adapter for a provider name.
func DefaultProviders(provider, token, secret string) Provider {
	switch provider {
	case "gitlab":
		return NewGitLab(token, secret)
	case "gitea":
		return NewGitea(token, []byte(secret))
	case "bitbucket":
		return NewBitbucket(token, []byte(secret))
	default:
		return NewGitHub(token, secret)
	}
}
