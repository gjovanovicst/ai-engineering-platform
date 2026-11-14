package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/deployments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// DeploymentService records environments and releases.
type DeploymentService interface {
	CreateEnvironment(ctx context.Context, orgID, projectID, name, url string) (deployments.Environment, error)
	ListEnvironments(ctx context.Context, orgID, projectID string) ([]deployments.Environment, error)
	Deploy(ctx context.Context, orgID, environmentID, releaseRef, userID string) (deployments.Deployment, error)
	ListDeployments(ctx context.Context, orgID, environmentID string, limit int) ([]deployments.Deployment, error)
	Approve(ctx context.Context, orgID, deploymentID, approverID string) (deployments.Deployment, error)
	Logs(ctx context.Context, orgID, deploymentID string) (string, error)
}

type deploymentHandlers struct {
	svc      DeploymentService
	projects ProjectService
	issuer   *auth.Issuer
	log      *slog.Logger
}

func newDeploymentHandlers(svc DeploymentService, projects ProjectService, issuer *auth.Issuer, log *slog.Logger) *deploymentHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &deploymentHandlers{svc: svc, projects: projects, issuer: issuer, log: log}
}

func (h *deploymentHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/environments", h.listEnvironments)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/environments", h.createEnvironment)
	mux.HandleFunc("GET /api/v1/orgs/{org}/environments/{environment}/deployments", h.listDeployments)
	mux.HandleFunc("POST /api/v1/orgs/{org}/environments/{environment}/deployments", h.deploy)
	mux.HandleFunc("POST /api/v1/orgs/{org}/deployments/{deployment}/approve", h.approve)
	mux.HandleFunc("GET /api/v1/orgs/{org}/deployments/{deployment}/logs", h.logs)
}

func (h *deploymentHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	return claims, true
}

func (h *deploymentHandlers) listEnvironments(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListEnvironments(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		h.log.Error("deployments.list_envs", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list environments")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, env := range items {
		data = append(data, map[string]any{
			"id": env.ID, "name": env.Name, "url": env.URL,
			"created_at": env.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *deploymentHandlers) createEnvironment(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	env, err := h.svc.CreateEnvironment(r.Context(), claims.OrgID, r.PathValue("project"), req.Name, req.URL)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": env.ID, "name": env.Name, "url": env.URL})
}

func (h *deploymentHandlers) listDeployments(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListDeployments(r.Context(), claims.OrgID, r.PathValue("environment"), 50)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": deploymentJSON(items)})
}

func (h *deploymentHandlers) deploy(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		ReleaseRef string `json:"release_ref"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	deployment, err := h.svc.Deploy(r.Context(), claims.OrgID, r.PathValue("environment"), req.ReleaseRef, claims.Subject)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, deploymentJSONOne(deployment))
}

func (h *deploymentHandlers) approve(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	deployment, err := h.svc.Approve(r.Context(), claims.OrgID, r.PathValue("deployment"), claims.Subject)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deploymentJSONOne(deployment))
}

func (h *deploymentHandlers) logs(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	log, err := h.svc.Logs(r.Context(), claims.OrgID, r.PathValue("deployment"))
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"log": log})
}

func (h *deploymentHandlers) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", "the environment already exists")
	case errors.Is(err, domain.ErrInvalidTransition):
		writeProblem(w, http.StatusConflict, "invalid_transition", "the deployment cannot be approved")
	default:
		h.log.Error("deployments", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "deployment operation failed")
	}
}

func deploymentJSON(items []deployments.Deployment) []map[string]any {
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, deploymentJSONOne(item))
	}
	return data
}

func deploymentJSONOne(d deployments.Deployment) map[string]any {
	out := map[string]any{
		"id": d.ID, "environment_id": d.EnvironmentID, "environment": d.Environment,
		"release_ref": d.ReleaseRef, "status": d.Status, "deployed_by": d.DeployedBy,
	}
	if d.ApprovedBy != "" {
		out["approved_by"] = d.ApprovedBy
	}
	if d.StartedAt != nil {
		out["started_at"] = d.StartedAt.UTC().Format(time.RFC3339)
	}
	if d.FinishedAt != nil {
		out["finished_at"] = d.FinishedAt.UTC().Format(time.RFC3339)
	}
	return out
}
