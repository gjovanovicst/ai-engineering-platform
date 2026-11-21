package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RepoLookup returns the repo connected to a project.
type RepoLookup interface {
	ByProject(ctx context.Context, orgID, projectID string) (domain.Repo, error)
}

// TaskLookup reads the task a run belongs to (for the PR body).
type TaskLookup interface {
	Get(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
}

// PRRecorder persists the created PR/MR.
type PRRecorder interface {
	RecordPullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
}

// ProviderFactory builds an authenticated provider.
type ProviderFactory func(provider, token, secret string) Provider

// WorkspacePusher publishes an exported workspace.
type WorkspacePusher interface {
	Push(ctx context.Context, archive []byte, req PushRequest) error
}

// Publisher pushes the development branch and opens a PR/MR.
type Publisher struct {
	Repos     RepoLookup
	Recorder  PRRecorder
	Pusher    WorkspacePusher
	Providers ProviderFactory
	Tokens    map[string]string // provider -> token
	// TokenFunc, when set, supplies tokens (e.g. a GitHub App installation
	// token that is refreshed automatically); it wins over Tokens.
	TokenFunc func(ctx context.Context, provider string) (string, error)
	// RepoTokenFunc supplies the per-repository credential. It takes
	// precedence over every organisation/global token; an error or an empty
	// token falls through to the next source.
	RepoTokenFunc func(ctx context.Context, repo domain.Repo) (string, error)
	// OrgTokenFunc takes precedence for per-org GitHub App tokens; a failure
	// falls back to the static token so a broken app does not stop delivery.
	OrgTokenFunc func(ctx context.Context, orgID, provider string) (string, error)
	Tasks        TaskLookup
	WebURL       string
	// CloneBases maps provider -> base URL used to build the push URL. It is
	// configurable so self-hosted GitLab/GitHub Enterprise work too.
	CloneBases map[string]string
}

// Publish implements runs.Publisher. A project without a repo is a no-op.
func (p *Publisher) Publish(ctx context.Context, run domain.Run, workspace []byte) (domain.PullRequest, error) {
	repo, err := p.Repos.ByProject(ctx, run.OrgID, run.ProjectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.PullRequest{}, nil
		}
		return domain.PullRequest{}, err
	}
	token, err := p.tokenFor(ctx, run.OrgID, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}

	branchID := run.ID
	if run.FixOfRunID != "" {
		branchID = run.FixOfRunID
	}
	branch := "aip/" + shortID(branchID)
	if err := p.Pusher.Push(ctx, workspace, PushRequest{
		RemoteURL: remoteURL(repo, token, p.CloneBases),
		Branch:    branch,
		Message:   "aip: " + shortID(run.ID),
	}); err != nil {
		return domain.PullRequest{}, err
	}

	title, body := "aip: "+shortID(run.ID), "Opened by the AI Engineering Platform."
	if p.Tasks != nil {
		if task, terr := p.Tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID); terr == nil {
			title = fmt.Sprintf("%s: %s", task.Ref, task.Title)
			body = prBody(task, run, p.WebURL)
		}
	}
	provider := p.Providers(string(repo.Provider), token, repo.WebhookSecret)
	if finder, ok := provider.(interface {
		FindOpenPullRequest(ctx context.Context, repo, head string) (int, string, bool, error)
	}); ok {
		if number, url, found, ferr := finder.FindOpenPullRequest(ctx, repo.Name, branch); ferr == nil && found {
			return p.Recorder.RecordPullRequest(ctx, domain.PullRequest{
				OrgID: run.OrgID, RepoID: repo.ID, TaskID: run.TaskID,
				Provider: repo.Provider, Number: number, URL: url,
			})
		}
	}
	pr, err := provider.CreatePullRequest(ctx, CreateRequest{
		Repo: repo.Name, Title: title,
		Body: body, Head: branch, Base: repo.DefaultBranch,
	})
	if err != nil {
		return domain.PullRequest{}, err
	}

	return p.Recorder.RecordPullRequest(ctx, domain.PullRequest{
		OrgID: run.OrgID, RepoID: repo.ID, TaskID: run.TaskID,
		Provider: repo.Provider, Number: pr.Number, URL: pr.URL,
	})
}

// Merge merges the PR/MR after a human approved it.
func (p *Publisher) Merge(ctx context.Context, orgID, projectID string, number int) error {
	repo, err := p.Repos.ByProject(ctx, orgID, projectID)
	if err != nil {
		return err
	}
	token, err := p.tokenFor(ctx, orgID, repo)
	if err != nil {
		return err
	}
	title := "aip: " + repo.Name + " #" + strconv.Itoa(number)
	return p.Providers(string(repo.Provider), token, repo.WebhookSecret).MergePullRequest(ctx, repo.Name, number, title)
}

// prBody renders the required PR sections from the task and the run.
func escapePipe(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ")
}

func prBody(task domain.Task, run domain.Run, webURL string) string {
	var plan struct {
		Steps []string `json:"steps"`
		Files []string `json:"files"`
		Risks []string `json:"risks"`
	}
	_ = json.Unmarshal(run.Plan, &plan)

	var out strings.Builder
	out.WriteString("## What\n")
	fmt.Fprintf(&out, "%s — %s\n\n", task.Ref, task.Title)
	if strings.TrimSpace(task.Description) != "" {
		fmt.Fprintf(&out, "%s\n\n", strings.TrimSpace(task.Description))
	}
	if len(task.Acceptance) > 0 {
		out.WriteString("Acceptance:\n")
		for _, ac := range task.Acceptance {
			fmt.Fprintf(&out, "- Given %s, when %s, then %s\n", ac.Given, ac.When, ac.Then)
		}
		out.WriteString("\n")
	}
	if len(run.AcceptanceProof) > 0 {
		out.WriteString("## Acceptance evidence\n")
		var proof struct {
			Evidence []struct {
				Criterion string `json:"criterion"`
				Kind      string `json:"kind"`
				Test      string `json:"test"`
				Reason    string `json:"reason"`
			} `json:"evidence"`
		}
		if err := json.Unmarshal(run.AcceptanceProof, &proof); err == nil && len(proof.Evidence) > 0 {
			out.WriteString("| Criterion | Evidence |\n|---|---|\n")
			for _, ev := range proof.Evidence {
				text := ev.Criterion
				var index int
				if _, err := fmt.Sscanf(ev.Criterion, "AC%d", &index); err == nil && index > 0 && index <= len(task.Acceptance) {
					ac := task.Acceptance[index-1]
					text = fmt.Sprintf("%s — Given %s, when %s, then %s", ev.Criterion, ac.Given, ac.When, ac.Then)
				}
				evidence := ev.Test
				if ev.Kind == "manual" {
					evidence = "manual: " + ev.Reason
				}
				out.WriteString("| " + escapePipe(text) + " | " + escapePipe(evidence) + " |\n")
			}
			out.WriteString("\n")
		}
	}
	out.WriteString("## Why\n")
	out.WriteString("Opened by the AI Engineering Platform from the approved plan.\n\n")
	out.WriteString("## Plan\n")
	if len(plan.Steps) == 0 {
		out.WriteString("_No plan steps were recorded._\n")
	} else {
		for i, step := range plan.Steps {
			fmt.Fprintf(&out, "%d. %s\n", i+1, step)
		}
	}
	if len(plan.Files) > 0 {
		out.WriteString("\nFiles: ")
		out.WriteString(strings.Join(plan.Files, ", "))
		out.WriteString("\n")
	}
	out.WriteString("\n## How tested\n")
	out.WriteString("- CI runs on this pull request (check runs are reported to the platform).\n")
	out.WriteString("- The agent ran the repository test command in an isolated sandbox before pushing.\n")
	out.WriteString("- Diff: ")
	fmt.Fprintf(&out, "%d file(s), %d bytes.\n", strings.Count(string(run.Diff), "diff --git"), len(run.Diff))
	out.WriteString("\n## Screenshot\n")
	out.WriteString("N/A for a non-visual change; a visual change attaches one.\n")
	out.WriteString("\n## Risk and rollback\n")
	if len(plan.Risks) == 0 {
		out.WriteString("Risk: low — see the plan. ")
	} else {
		for _, risk := range plan.Risks {
			fmt.Fprintf(&out, "- %s\n", risk)
		}
	}
	out.WriteString("Rollback: revert this merge commit; no migration is dropped automatically.\n")
	if webURL != "" {
		fmt.Fprintf(&out, "\nRun: %s/projects/%s/runs/%s\n", strings.TrimRight(webURL, "/"), run.ProjectID, run.ID)
	}
	return out.String()
}

func (p *Publisher) tokenFor(ctx context.Context, orgID string, repo domain.Repo) (string, error) {
	if p.RepoTokenFunc != nil {
		if token, err := p.RepoTokenFunc(ctx, repo); err == nil && token != "" {
			return token, nil
		}
	}
	if p.OrgTokenFunc != nil && repo.Provider == domain.ProviderGitHub {
		if token, err := p.OrgTokenFunc(ctx, orgID, string(repo.Provider)); err == nil && token != "" {
			return token, nil
		}
	}
	return p.token(ctx, string(repo.Provider))
}

func (p *Publisher) token(ctx context.Context, provider string) (string, error) {
	if p.TokenFunc != nil {
		token, err := p.TokenFunc(ctx, provider)
		if err != nil {
			return "", err
		}
		if token == "" {
			return "", fmt.Errorf("git: no token configured for %s", provider)
		}
		return token, nil
	}
	token := p.Tokens[provider]
	if token == "" {
		return "", fmt.Errorf("git: no token configured for %s", provider)
	}
	return token, nil
}

func remoteURL(repo domain.Repo, token string, bases map[string]string) string {
	user := "x-access-token"
	base := "https://github.com"
	if repo.Provider == domain.ProviderGitLab {
		user = "oauth2"
		base = "https://gitlab.com"
	}
	if repo.Provider == domain.ProviderGitea {
		user = "oauth2"
		base = "https://gitea.com"
	}
	if repo.Provider == domain.ProviderBitbucket {
		user = "x-token-auth"
		base = "https://bitbucket.org"
	}
	if custom := bases[string(repo.Provider)]; custom != "" {
		base = custom
	}
	if strings.HasPrefix(base, "file://") {
		return strings.TrimRight(base, "/") + "/" + repo.Name + ".git"
	}
	// http(s):// -> inject credentials after the scheme
	scheme := "https://"
	if strings.HasPrefix(base, "http://") {
		scheme = "http://"
		base = strings.TrimPrefix(base, "http://")
	} else {
		base = strings.TrimPrefix(base, "https://")
	}
	return scheme + user + ":" + token + "@" + strings.TrimRight(base, "/") + "/" + repo.Name + ".git"
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Rebase updates the PR branch from its base when the provider supports it.
func (p *Publisher) Rebase(ctx context.Context, orgID, projectID string, number int) error {
	repo, err := p.Repos.ByProject(ctx, orgID, projectID)
	if err != nil {
		return err
	}
	token, err := p.tokenFor(ctx, orgID, repo)
	if err != nil {
		return err
	}
	provider := p.Providers(string(repo.Provider), token, repo.WebhookSecret)
	if rebaser, ok := provider.(interface {
		RebasePullRequest(ctx context.Context, repo string, number int) error
	}); ok {
		return rebaser.RebasePullRequest(ctx, repo.Name, number)
	}
	return nil
}
