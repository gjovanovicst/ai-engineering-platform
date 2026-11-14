package domain

import (
	"strings"
	"time"
)

// GitProvider is a supported source host.
type GitProvider string

const (
	ProviderGitHub    GitProvider = "github"
	ProviderGitLab    GitProvider = "gitlab"
	ProviderGitea     GitProvider = "gitea"
	ProviderBitbucket GitProvider = "bitbucket"
)

// Valid reports whether p is supported.
func (p GitProvider) Valid() bool {
	return p == ProviderGitHub || p == ProviderGitLab || p == ProviderGitea || p == ProviderBitbucket
}

// ParseGitProvider validates an API value.
func ParseGitProvider(raw string) (GitProvider, error) {
	p := GitProvider(strings.ToLower(strings.TrimSpace(raw)))
	if !p.Valid() {
		return "", invalid("provider", "provider must be github or gitlab")
	}
	return p, nil
}

// Repo is a connected source repository.
type Repo struct {
	ID            string
	OrgID         string
	ProjectID     string
	Provider      GitProvider
	Name          string
	ExternalID    string
	DefaultBranch string
	WebhookSecret string
	CreatedAt     time.Time
}

// PullRequest links a PR/MR to the task that produced it.
type PullRequest struct {
	ID        string
	OrgID     string
	RepoID    string
	TaskID    string
	Provider  GitProvider
	Number    int
	URL       string
	State     string
	TaskRef   string
	TaskTitle string
	RepoName  string
}
