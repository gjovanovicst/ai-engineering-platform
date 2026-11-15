package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RepoService persists connected repositories and PR links.
type RepoService interface {
	Create(ctx context.Context, r domain.Repo) (domain.Repo, error)
	List(ctx context.Context, orgID, projectID string) ([]domain.Repo, error)
	Get(ctx context.Context, orgID, projectID, repoID string) (domain.Repo, error)
	ForWebhook(ctx context.Context, repoID string) (domain.Repo, error)
	ForWebhookName(ctx context.Context, provider, name string) (domain.Repo, error)
	RecordPullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error)
	PullRequestForMerge(ctx context.Context, orgID, repoID string, number int) (domain.PullRequest, error)
	SetPullRequestState(ctx context.Context, orgID, id, state string) error
	ApprovedForPR(ctx context.Context, orgID, pullRequestID string) (bool, error)
	PullRequestsForProject(ctx context.Context, orgID, projectID string, limit int) ([]domain.PullRequest, error)
	PullRequest(ctx context.Context, orgID, id string) (domain.PullRequest, error)
}

type repoHandlers struct {
	repos    RepoService
	creds    RepoCredentialService
	projects ProjectService
	issuer   *auth.Issuer
	log      *slog.Logger
}

func newRepoHandlers(repos RepoService, creds RepoCredentialService, projects ProjectService, issuer *auth.Issuer, log *slog.Logger) *repoHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &repoHandlers{repos: repos, creds: creds, projects: projects, issuer: issuer, log: log}
}

func (h *repoHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/repos", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/repos", h.list)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/pull-requests", h.pullRequests)
	mux.HandleFunc("GET /api/v1/orgs/{org}/pull-requests/{pr}", h.pullRequest)
}

// registerCredentialRoutes is separate because the credential store is optional
// in tests and in deployments that do not set AIP_SECRETS_KEY.
func (h *repoHandlers) registerCredentialRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/repos/{repo}/credential", h.credential)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/repos/{repo}/credential", h.setCredential)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/projects/{project}/repos/{repo}/credential", h.deleteCredential)
}

func (h *repoHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if !roleAtLeast(claims.Role, "member") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return
	}
	projectID := r.PathValue("project")
	if _, err := h.projects.Get(r.Context(), claims.OrgID, projectID); err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	var req struct {
		Provider      string `json:"provider"`
		Name          string `json:"name"`
		ExternalID    string `json:"external_id"`
		DefaultBranch string `json:"default_branch"`
		WebhookSecret string `json:"webhook_secret"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	provider, err := domain.ParseGitProvider(req.Provider)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "provider must be github, gitlab, gitea or bitbucket")
		return
	}
	if req.Name == "" || req.WebhookSecret == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "name and webhook_secret are required")
		return
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	repo, err := h.repos.Create(r.Context(), domain.Repo{
		OrgID: claims.OrgID, ProjectID: projectID, Provider: provider, Name: req.Name,
		ExternalID: req.ExternalID, DefaultBranch: req.DefaultBranch, WebhookSecret: req.WebhookSecret,
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeProblem(w, http.StatusConflict, "conflict", "this repository is already connected")
			return
		}
		h.log.Error("repos.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not connect the repository")
		return
	}
	writeJSON(w, http.StatusCreated, repoJSON(repo))
}

func (h *repoHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	items, err := h.repos.List(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		h.log.Error("repos.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list repositories")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, repo := range items {
		data = append(data, repoJSON(repo))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *repoHandlers) pullRequest(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	pr, err := h.repos.PullRequest(r.Context(), claims.OrgID, r.PathValue("pr"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "pull request not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": pr.ID, "provider": pr.Provider, "number": pr.Number, "url": pr.URL,
		"state": pr.State, "task_id": pr.TaskID, "repo_name": pr.RepoName,
		"checks": []any{}, "files": []any{},
		"review": map[string]any{"issues": []any{}},
		"linked": map[string]any{
			"task_id": pr.TaskID, "task_ref": pr.TaskRef, "task_title": pr.TaskTitle,
		},
	})
}

func (h *repoHandlers) pullRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	items, err := h.repos.PullRequestsForProject(r.Context(), claims.OrgID, r.PathValue("project"), 50)
	if err != nil {
		h.log.Error("repos.pull_requests", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list pull requests")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, pr := range items {
		data = append(data, map[string]any{
			"id": pr.ID, "provider": pr.Provider, "number": pr.Number,
			"url": pr.URL, "state": pr.State, "task_id": pr.TaskID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func repoJSON(repo domain.Repo) map[string]any {
	return map[string]any{
		"id": repo.ID, "provider": repo.Provider, "name": repo.Name,
		"default_branch": repo.DefaultBranch, "created_at": repo.CreatedAt.UTC().Format(time.RFC3339),
	}
}
