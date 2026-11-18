package git

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RepoGetter loads a repository by id (implemented by the postgres repo store).
type RepoGetter interface {
	Get(ctx context.Context, orgID, projectID, repoID string) (domain.Repo, error)
}

// CheckLogs downloads CI job logs on the host for a fix-CI run; the sandbox
// never gets network access. GitHub Actions job logs are fetched with the
// repository token (the per-repository credential wins).
type CheckLogs struct {
	Repos         RepoGetter
	Tokens        map[string]string
	RepoTokenFunc func(ctx context.Context, repo domain.Repo) (string, error)
	TokenFunc     func(ctx context.Context, provider string) (string, error)
	APIURL        string
	HTTP          *http.Client
}

var jobPathPattern = regexp.MustCompile(`/actions/runs/\d+/job/(\d+)`)

// parseJobID extracts the job id from a GitHub Actions job page URL.
func parseJobID(checkURL string) string {
	m := jobPathPattern.FindStringSubmatch(checkURL)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

// Fetch returns the log of a failed GitHub Actions check.
func (c *CheckLogs) Fetch(ctx context.Context, orgID, projectID, repoID, checkURL string) (string, error) {
	repo, err := c.Repos.Get(ctx, orgID, projectID, repoID)
	if err != nil {
		return "", err
	}
	if repo.Provider != domain.ProviderGitHub {
		return "", fmt.Errorf("check logs: provider %s is not supported", repo.Provider)
	}
	jobID := parseJobID(checkURL)
	if jobID == "" {
		return "", fmt.Errorf("check logs: no GitHub Actions job in %q", checkURL)
	}
	token := c.Tokens[string(repo.Provider)]
	if c.RepoTokenFunc != nil {
		if fresh, terr := c.RepoTokenFunc(ctx, repo); terr == nil && fresh != "" {
			token = fresh
		}
	}
	if c.TokenFunc != nil {
		fresh, terr := c.TokenFunc(ctx, string(repo.Provider))
		if terr != nil {
			return "", terr
		}
		token = fresh
	}
	api := strings.TrimRight(c.APIURL, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/actions/jobs/%s/logs", api, repo.Name, jobID)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("check logs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return "", fmt.Errorf("check logs: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// The failure is at the end; keep the last two megabytes.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("check logs: read: %w", err)
	}
	return string(body), nil
}
