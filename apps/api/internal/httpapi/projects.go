package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	gitadapters "github.com/gjovanovicst/ai-engineering-platform/internal/adapters/git"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/mergequeue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/policydiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/httpapi/oapi"
)

// ProjectService is the subset of projects.Service the HTTP layer needs.
type ProjectService interface {
	Create(ctx context.Context, in projects.CreateInput) (domain.Project, error)
	Get(ctx context.Context, orgID, id string) (domain.Project, error)
	List(ctx context.Context, orgID string, limit int) ([]domain.Project, error)
	UpdateRequiredChecks(ctx context.Context, orgID, id string, checks []string) (domain.Project, error)
	SetDesignStandard(ctx context.Context, orgID, projectID, standardID string) (domain.Project, error)
	DesignStandard(ctx context.Context, orgID, projectID string) (projects.ProjectStandard, error)
	ListComponents(ctx context.Context, orgID, projectID string) ([]projects.ProjectComponent, error)
	ReplaceComponents(ctx context.Context, orgID, projectID, actorID string, selections []projects.ComponentSelection) (int, error)
	SetE2ECommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	SetTestCommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	Update(ctx context.Context, orgID, id string, in projects.UpdateInput) (domain.Project, error)
	SetVisualCommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	SetMCPConfig(ctx context.Context, orgID, id, config string) (domain.Project, error)
	SetVisualPolicy(ctx context.Context, orgID, id string, threshold float64, masks string) (domain.Project, error)
	SetPolicy(ctx context.Context, orgID, id string, maxBytes int, paths []string) (domain.Project, error)
	SetChangeWindow(ctx context.Context, orgID, id, window string) (domain.Project, error)
	SetAllowedModels(ctx context.Context, orgID, id string, models []string) (domain.Project, error)
	SetAllowedTools(ctx context.Context, orgID, id string, tools []string) (domain.Project, error)
	SetRequiredApprovals(ctx context.Context, orgID, id string, n int) (domain.Project, error)
	SetPluginChecks(ctx context.Context, orgID, id, checks string) (domain.Project, error)
	Delete(ctx context.Context, orgID, id, actorID, confirmName string) error
}

// GitHubAppTokenMinter mints an installation token for branch protection.
// ObjectStore stores visual baselines outside Postgres.
type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// MergeQueueReader lists queued merges for a project.
type MergeQueueReader interface {
	List(ctx context.Context, orgID, projectID string) ([]mergequeue.Item, error)
}

type GitHubAppTokenMinter interface {
	InstallationToken(ctx context.Context, orgID string) (string, error)
}

// RepoTokenReader resolves the per-repository git credential.
type RepoTokenReader interface {
	Token(ctx context.Context, orgID, repoID string) (string, error)
}

type projectHandlers struct {
	svc          ProjectService
	repos        RepoService
	testCommand  string
	agentBackend string
	githubToken  string
	gitlabToken  string
	appTokens    GitHubAppTokenMinter
	repoTokens   RepoTokenReader
	objects      ObjectStore
	mergeQueue   MergeQueueReader
	access       ProjectAccessReader
	githubApp    bool
	issuer       *auth.Issuer
	log          *slog.Logger
}

func newProjectHandlers(svc ProjectService, repos RepoService, issuer *auth.Issuer, testCommand, agentBackend, githubToken, gitlabToken string, appTokens GitHubAppTokenMinter, objects ObjectStore, mergeQueue MergeQueueReader, access ProjectAccessReader, githubApp bool, log *slog.Logger) *projectHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &projectHandlers{svc: svc, repos: repos, issuer: issuer, testCommand: testCommand, agentBackend: agentBackend, githubToken: githubToken, gitlabToken: gitlabToken, appTokens: appTokens, objects: objects, mergeQueue: mergeQueue, access: access, githubApp: githubApp, log: log}
}

// withRepoTokens enables per-repository credentials (repo -> app -> global).
func (h *projectHandlers) withRepoTokens(reader RepoTokenReader) *projectHandlers {
	h.repoTokens = reader
	return h
}

// tokenForRepo resolves the git token in priority order: the repository
// credential, then the GitHub App installation token, then the global token.
func (h *projectHandlers) tokenForRepo(ctx context.Context, orgID string, repo domain.Repo) string {
	if h.repoTokens != nil {
		if token, err := h.repoTokens.Token(ctx, orgID, repo.ID); err == nil && token != "" {
			return token
		}
	}
	if repo.Provider == domain.ProviderGitHub {
		if h.appTokens != nil {
			if token, err := h.appTokens.InstallationToken(ctx, orgID); err == nil && token != "" {
				return token
			}
		}
		return h.githubToken
	}
	return h.gitlabToken
}

func (h *projectHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}", h.get)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/required-checks", h.updateRequiredChecks)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/e2e", h.setE2ECommand)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/test-command", h.setTestCommand)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/visual", h.setVisualCommand)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/mcp", h.setMCPConfig)
	mux.HandleFunc("PATCH /api/v1/orgs/{org}/projects/{project}", h.update)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/protection", h.protectBranch)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/merge-queue", h.mergeQueueList)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/visual-diff", h.visualDiff)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/visual-baseline", h.putVisualBaseline)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/visual-baseline", h.getVisualBaseline)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/visual-policy", h.setVisualPolicy)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy", h.setPolicy)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy/window", h.setChangeWindow)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy/models", h.setAllowedModels)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy/tools", h.setAllowedTools)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy/approvals", h.setRequiredApprovals)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/policy/plugins", h.setPluginChecks)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/projects/{project}", h.delete)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/policy/evaluate", h.evaluatePolicy)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/policy/evaluate-pr", h.evaluatePullRequestPolicy)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/preflight", h.preflight)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/standards", h.designStandard)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/standards", h.setDesignStandard)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/components", h.projectComponents)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/components", h.setProjectComponents)
}

// authorize resolves the caller and enforces tenant + role. A mismatched org
// returns 404 so that the API does not disclose other tenants' existence.
func (h *projectHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return nil, false
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	return claims, true
}

// projectVisible hides projects with explicit members from non-members.
func (h *projectHandlers) projectVisible(ctx context.Context, claims *auth.Claims, projectID string) bool {
	if claims.Role == "owner" || claims.Role == "admin" || h.access == nil {
		return true
	}
	result, err := h.access.Access(ctx, claims.OrgID, projectID, claims.Subject)
	if err != nil {
		return false
	}
	return result.Members == 0 || result.Role != ""
}

// update changes the project's name and description (the settings keep their
// own routes).
func (h *projectHandlers) update(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	p, err := h.svc.Update(r.Context(), claims.OrgID, r.PathValue("project"), projects.UpdateInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.update", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not update the project")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(p))
}

func (h *projectHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req oapi.ProjectInput
	if !decode(w, r, 1<<20, &req) {
		return
	}
	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	p, err := h.svc.Create(r.Context(), projects.CreateInput{
		OrgID: claims.OrgID, Name: req.Name, Description: description,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "a project with this slug already exists")
		default:
			h.log.Error("projects.create", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not create the project")
		}
		return
	}
	writeJSON(w, http.StatusCreated, projectJSON(p))
}

func (h *projectHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return
		}
		limit = n
	}

	items, err := h.svc.List(r.Context(), claims.OrgID, limit)
	if err != nil {
		h.log.Error("projects.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list projects")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, p := range items {
		if !h.projectVisible(r.Context(), claims, p.ID) {
			continue
		}
		data = append(data, projectJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": nil})
}

func (h *projectHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	p, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		h.log.Error("projects.get", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the project")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(p))
}

func (h *projectHandlers) setE2ECommand(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetE2ECommand(r.Context(), claims.OrgID, r.PathValue("project"), req.Command)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.e2e", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the e2e command")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setTestCommand(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Command     string `json:"command"`
		TestCommand string `json:"test_command"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	// The round trip is symmetric: accept the field name the response returns.
	if req.Command == "" {
		req.Command = req.TestCommand
	}
	req.Command = strings.TrimSpace(req.Command)
	if len(req.Command) > 500 || strings.ContainsAny(req.Command, "\n\r") {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "the test command must be one line of at most 500 characters")
		return
	}
	project, err := h.svc.SetTestCommand(r.Context(), claims.OrgID, r.PathValue("project"), req.Command)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		default:
			h.log.Error("projects.test_command", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the test command")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) protectBranch(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	project, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	repos, err := h.repos.List(r.Context(), claims.OrgID, project.ID)
	if err != nil || len(repos) == 0 {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "connect a repository first")
		return
	}
	repo := repos[0]
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	if repo.Provider == domain.ProviderGitLab {
		token := h.tokenForRepo(r.Context(), claims.OrgID, repo)
		if token == "" {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "no GitLab token configured")
			return
		}
		client := gitadapters.NewGitLab(token, "")
		if err := client.ProtectBranch(r.Context(), repo.Name, branch, project.RequiredChecks); err != nil {
			h.log.Warn("projects.protection", slog.Any("err", err))
			writeProblem(w, http.StatusBadGateway, "github_error", "could not protect the branch: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"protected": true, "provider": "gitlab", "repo": repo.Name, "branch": branch})
		return
	}
	if repo.Provider != domain.ProviderGitHub {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "branch protection is not supported for this provider yet")
		return
	}
	token := h.tokenForRepo(r.Context(), claims.OrgID, repo)
	if token == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "no GitHub token or app installation available")
		return
	}
	client := gitadapters.NewGitHub(token, "")
	if err := client.ProtectBranch(r.Context(), repo.Name, branch, project.RequiredChecks); err != nil {
		h.log.Warn("projects.protection", slog.Any("err", err))
		writeProblem(w, http.StatusBadGateway, "github_error", "could not protect the branch: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"protected": true, "repo": repo.Name, "branch": branch, "checks": project.RequiredChecks})
}

func (h *projectHandlers) setMCPConfig(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Config string `json:"config"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetMCPConfig(r.Context(), claims.OrgID, r.PathValue("project"), req.Config)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.mcp", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the MCP config")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setVisualCommand(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetVisualCommand(r.Context(), claims.OrgID, r.PathValue("project"), req.Command)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.visual", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the visual command")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) updateRequiredChecks(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Checks []string `json:"checks"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.UpdateRequiredChecks(r.Context(), claims.OrgID, r.PathValue("project"), req.Checks)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		default:
			h.log.Error("projects.required_checks", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the checks")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func projectJSON(p domain.Project) map[string]any {
	return map[string]any{
		"id":                        p.ID,
		"org_id":                    p.OrgID,
		"name":                      p.Name,
		"slug":                      p.Slug,
		"description":               p.Description,
		"required_checks":           p.RequiredChecks,
		"e2e_command":               p.E2ECommand,
		"test_command":              p.TestCommand,
		"visual_command":            p.VisualCommand,
		"mcp_servers":               p.MCPServers,
		"agent_plugin_id":           p.AgentPluginID,
		"visual_threshold":          p.VisualThreshold,
		"visual_masks":              p.VisualMasks,
		"policy_max_diff_bytes":     p.PolicyMaxDiffBytes,
		"policy_forbidden_paths":    p.PolicyForbiddenPaths,
		"policy_change_window":      p.PolicyChangeWindow,
		"policy_allowed_models":     p.PolicyAllowedModels,
		"policy_allowed_tools":      p.PolicyAllowedTools,
		"policy_required_approvals": p.PolicyRequiredApprovals,
		"plugin_checks":             p.PluginChecks,
		"design_standard_id":        p.DesignStandardID,
		"design_standard_version":   p.DesignStandardVersion,
		"status":                    string(p.Status),
		"created_at":                p.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":                p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *projectHandlers) designStandard(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	standard, err := h.svc.DesignStandard(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"design_standard": nil})
			return
		}
		h.log.Error("projects.design_standard", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the standard")
		return
	}
	out := map[string]any{
		"id": standard.ID, "name": standard.Name, "version": standard.Version, "sha256": standard.SHA256,
	}
	if standard.PublishedAt != nil {
		out["published_at"] = standard.PublishedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, map[string]any{"design_standard": out})
}

func (h *projectHandlers) setDesignStandard(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		DesignStandardID string `json:"design_standard_id"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetDesignStandard(r.Context(), claims.OrgID, r.PathValue("project"), req.DesignStandardID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		default:
			h.log.Error("projects.set_design_standard", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the standard")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) projectComponents(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListComponents(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		h.log.Error("projects.components", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load components")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{
			"id": item.ID, "name": item.Name, "category": item.Category, "status": item.Status,
			"import_path": item.ImportPath, "package": item.Package, "version": item.Version,
			"mode": item.Mode, "reason": item.Reason,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *projectHandlers) setProjectComponents(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		ComponentIDs []string `json:"component_ids"`
		Components   []struct {
			ComponentID string `json:"component_id"`
			Mode        string `json:"mode"`
			Reason      string `json:"reason"`
		} `json:"components"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	selections := make([]projects.ComponentSelection, 0, len(req.Components)+len(req.ComponentIDs))
	for _, id := range req.ComponentIDs {
		selections = append(selections, projects.ComponentSelection{ID: id, Mode: "allow"})
	}
	for _, entry := range req.Components {
		selections = append(selections, projects.ComponentSelection{ID: entry.ComponentID, Mode: entry.Mode, Reason: entry.Reason})
	}
	count, err := h.svc.ReplaceComponents(r.Context(), claims.OrgID, r.PathValue("project"), claims.Subject, selections)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.set_components", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save components")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

type preflightCheck struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

func (h *projectHandlers) preflight(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	project, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	checks := []preflightCheck{
		{Key: "test_command", Label: "Test command", Status: "ok", Detail: h.testCommand},
	}
	if h.testCommand == "" {
		checks[0] = preflightCheck{Key: "test_command", Label: "Test command", Status: "fail", Detail: "No test command configured", Fix: "Set AIP_TEST_COMMAND in the environment"}
	}
	repos, rerr := h.repos.List(r.Context(), claims.OrgID, project.ID)
	if rerr != nil {
		checks = append(checks, preflightCheck{Key: "repository", Label: "Repository", Status: "fail", Detail: "Could not read repositories"})
	} else if len(repos) == 0 {
		checks = append(checks, preflightCheck{Key: "repository", Label: "Repository", Status: "fail", Detail: "No repository connected", Fix: "Connect a repository in Delivery → Repositories"})
	} else {
		checks = append(checks, preflightCheck{Key: "repository", Label: "Repository", Status: "ok", Detail: repos[0].Name})
	}
	if project.DesignStandardID == "" {
		checks = append(checks, preflightCheck{Key: "design_standard", Label: "Design standard", Status: "warn", Detail: "No published standard assigned", Fix: "Assign one in Design"})
	} else {
		checks = append(checks, preflightCheck{Key: "design_standard", Label: "Design standard", Status: "ok", Detail: project.DesignStandardID})
	}
	if len(project.RequiredChecks) == 0 {
		checks = append(checks, preflightCheck{Key: "required_checks", Label: "Required checks", Status: "warn", Detail: "No required checks", Fix: "Add test/e2e in Delivery → Checks"})
	} else {
		checks = append(checks, preflightCheck{Key: "required_checks", Label: "Required checks", Status: "ok", Detail: strings.Join(project.RequiredChecks, ", ")})
	}
	if project.E2ECommand == "" {
		checks = append(checks, preflightCheck{Key: "e2e", Label: "E2E command", Status: "warn", Detail: "No e2e command (optional)"})
	} else {
		checks = append(checks, preflightCheck{Key: "e2e", Label: "E2E command", Status: "ok", Detail: project.E2ECommand})
	}
	if project.MCPServers == "" {
		checks = append(checks, preflightCheck{Key: "mcp", Label: "MCP servers", Status: "ok", Detail: "none configured (optional)"})
	} else {
		var servers []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(project.MCPServers), &servers); err != nil {
			checks = append(checks, preflightCheck{Key: "mcp", Label: "MCP servers", Status: "fail", Detail: "Invalid MCP JSON", Fix: "Fix the JSON in Delivery → Checks → MCP servers"})
		} else {
			valid := true
			for _, server := range servers {
				if strings.TrimSpace(server.Name) == "" || strings.TrimSpace(server.Command) == "" {
					valid = false
				}
			}
			if !valid {
				checks = append(checks, preflightCheck{Key: "mcp", Label: "MCP servers", Status: "fail", Detail: "A server is missing name or command", Fix: "Add both fields in Delivery → Checks"})
			} else {
				checks = append(checks, preflightCheck{Key: "mcp", Label: "MCP servers", Status: "ok", Detail: fmt.Sprintf("%d configured", len(servers))})
			}
		}
	}
	if h.agentBackend == "" {
		checks = append(checks, preflightCheck{Key: "agent", Label: "Agent backend", Status: "fail", Detail: "No agent backend configured"})
	} else {
		checks = append(checks, preflightCheck{Key: "agent", Label: "Agent backend", Status: "ok", Detail: h.agentBackend})
	}
	status := "ok"
	if h.githubApp {
		checks = append(checks, preflightCheck{Key: "github_app", Label: "GitHub App", Status: "ok", Detail: "configured"})
	} else {
		checks = append(checks, preflightCheck{Key: "github_app", Label: "GitHub App", Status: "warn", Detail: "Not configured; PAT webhooks are used"})
	}
	ready := true
	for _, check := range checks {
		if check.Status == "fail" {
			ready = false
			status = "fail"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ready": ready, "status": status, "checks": checks})
}

func (h *projectHandlers) visualDiff(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, "member"); !ok {
		return
	}
	//nolint:gosec // the request is capped below; multipart is the upload format
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "expected a multipart upload")
		return
	}
	read := func(field string) ([]byte, error) {
		file, _, err := r.FormFile(field)
		if err != nil {
			return nil, err
		}
		defer func() { _ = file.Close() }()
		return io.ReadAll(io.LimitReader(file, 8<<20))
	}
	base, err := read("baseline")
	if err != nil {
		if name := strings.TrimSpace(r.FormValue("name")); name != "" && h.objects != nil {
			base, err = h.objects.Get(r.Context(), visualBaselineKey(r.PathValue("project"), name))
		}
		if err != nil || len(base) == 0 {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "baseline image or a stored name is required")
			return
		}
	}
	actual, err := read("actual")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "actual image is required")
		return
	}
	threshold := 0.01
	if raw := r.FormValue("threshold"); raw != "" {
		if parsed, perr := strconv.ParseFloat(raw, 64); perr == nil {
			threshold = parsed
		}
	}
	var masks []visualdiff.Rect
	if raw := r.FormValue("masks"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &masks)
	}
	result, err := visualdiff.ComparePNG(bytes.NewReader(base), bytes.NewReader(actual), threshold, masks)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func visualBaselineKey(projectID, name string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	return "visual/" + projectID + "/" + clean + ".png"
}

func (h *projectHandlers) delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		ConfirmName string `json:"confirm_name"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if err := h.svc.Delete(r.Context(), claims.OrgID, r.PathValue("project"), claims.Subject, req.ConfirmName); err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		h.log.Error("projects.delete", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not delete the project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *projectHandlers) setPluginChecks(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Checks string `json:"checks"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetPluginChecks(r.Context(), claims.OrgID, r.PathValue("project"), req.Checks)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the plugin checks")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setRequiredApprovals(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Required int `json:"required"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetRequiredApprovals(r.Context(), claims.OrgID, r.PathValue("project"), req.Required)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the approval rule")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setAllowedTools(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Tools []string `json:"tools"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetAllowedTools(r.Context(), claims.OrgID, r.PathValue("project"), req.Tools)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the allowed tools")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setAllowedModels(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Models []string `json:"models"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetAllowedModels(r.Context(), claims.OrgID, r.PathValue("project"), req.Models)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the allowed models")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setChangeWindow(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Window string `json:"window"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetChangeWindow(r.Context(), claims.OrgID, r.PathValue("project"), req.Window)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the change window")
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) evaluatePolicy(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	project, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	var req struct {
		Diff string `json:"diff"`
	}
	if !decode(w, r, 5<<20, &req) {
		return
	}
	violations := policydiff.Violations(req.Diff, project.PolicyMaxDiffBytes, project.PolicyForbiddenPaths)
	if violations == nil {
		violations = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"passed": len(violations) == 0, "violations": violations,
		"max_diff_bytes": project.PolicyMaxDiffBytes, "forbidden_paths": project.PolicyForbiddenPaths,
	})
}

func (h *projectHandlers) evaluatePullRequestPolicy(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	project, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	repos, rerr := h.repos.List(r.Context(), claims.OrgID, project.ID)
	if rerr != nil || len(repos) == 0 {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "connect a repository first")
		return
	}
	var req struct {
		Number int `json:"number"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if req.Number <= 0 {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "PR number is required")
		return
	}
	repo := repos[0]
	token := h.tokenForRepo(r.Context(), claims.OrgID, repo)
	var diff string
	switch repo.Provider {
	case domain.ProviderGitHub:
		if token == "" {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "no GitHub token or app installation available")
			return
		}
		diff, err = gitadapters.NewGitHub(token, "").PullRequestDiff(r.Context(), repo.Name, req.Number)
	case domain.ProviderGitLab:
		if token == "" {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "no GitLab token configured")
			return
		}
		diff, err = gitadapters.NewGitLab(token, "").PullRequestDiff(r.Context(), repo.Name, req.Number)
	default:
		writeProblem(w, http.StatusBadRequest, "validation_failed", "provider does not support PR diff fetch")
		return
	}
	if err != nil {
		h.log.Warn("projects.policy_pr", slog.Any("err", err))
		writeProblem(w, http.StatusBadGateway, "provider_error", "could not fetch the PR diff")
		return
	}
	violations := policydiff.Violations(diff, project.PolicyMaxDiffBytes, project.PolicyForbiddenPaths)
	if violations == nil {
		violations = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"passed": len(violations) == 0, "violations": violations, "number": req.Number,
		"diff_bytes": len(diff), "max_diff_bytes": project.PolicyMaxDiffBytes, "forbidden_paths": project.PolicyForbiddenPaths,
	})
}

func (h *projectHandlers) setPolicy(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		MaxDiffBytes int      `json:"max_diff_bytes"`
		Forbidden    []string `json:"forbidden_paths"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetPolicy(r.Context(), claims.OrgID, r.PathValue("project"), req.MaxDiffBytes, req.Forbidden)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.policy", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the policy")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) setVisualPolicy(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Threshold float64 `json:"threshold"`
		Masks     string  `json:"masks"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.svc.SetVisualPolicy(r.Context(), claims.OrgID, r.PathValue("project"), req.Threshold, req.Masks)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("projects.visual_policy", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the visual policy")
		}
		return
	}
	writeJSON(w, http.StatusOK, projectJSON(project))
}

func (h *projectHandlers) getVisualBaseline(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, "viewer"); !ok {
		return
	}
	if h.objects == nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "S3 is not configured")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	data, err := h.objects.Get(r.Context(), visualBaselineKey(r.PathValue("project"), name))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "baseline not found")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(data) //nolint:gosec // PNG bytes, Content-Type is fixed to image/png
}

func (h *projectHandlers) putVisualBaseline(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, "admin"); !ok {
		return
	}
	if h.objects == nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "S3 is required for visual baselines")
		return
	}
	//nolint:gosec // capped upload; multipart is the format
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "expected a multipart upload")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "file is required")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, 8<<20))
	if err != nil || len(data) == 0 {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "could not read the file")
		return
	}
	key := visualBaselineKey(r.PathValue("project"), name)
	if err := h.objects.Put(r.Context(), key, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		h.log.Error("projects.visual_baseline", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not store the baseline")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "key": key, "size_bytes": len(data)})
}

func (h *projectHandlers) mergeQueueList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorize(w, r, "viewer"); !ok {
		return
	}
	if h.mergeQueue == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	items, err := h.mergeQueue.List(r.Context(), r.PathValue("org"), r.PathValue("project"))
	if err != nil {
		h.log.Error("projects.merge_queue", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read the merge queue")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id": item.ID, "number": item.Number, "status": item.Status, "error": item.Error,
			"run_id": item.RunID, "task_id": item.TaskID, "created_at": item.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
