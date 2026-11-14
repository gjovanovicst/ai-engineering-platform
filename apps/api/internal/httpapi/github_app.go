package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/githubsetup"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// GitHubAppService is the manifest onboarding use case.
type GitHubAppService interface {
	CreateManifest(ctx context.Context, orgID string) (githubsetup.ManifestResult, error)
	Complete(ctx context.Context, state, code string) (githubsetup.Status, error)
	Status(ctx context.Context, orgID string) (githubsetup.Status, error)
	SetInstallation(ctx context.Context, orgID string, installationID int64) (githubsetup.Status, error)
	WebhookSecret(ctx context.Context, orgID string) (string, error)
	InstallationToken(ctx context.Context, orgID string) (string, error)
	Setup(ctx context.Context, state string, installationID int64) (githubsetup.Status, error)
}

type githubAppHandlers struct {
	svc    GitHubAppService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newGithubAppHandlers(svc GitHubAppService, issuer *auth.Issuer, log *slog.Logger) *githubAppHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &githubAppHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *githubAppHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/github-app", h.status)
	mux.HandleFunc("POST /api/v1/orgs/{org}/github-app/manifest", h.manifest)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/github-app/installation", h.installation)
	mux.HandleFunc("GET /api/v1/github-app/callback", h.callback)
	mux.HandleFunc("GET /api/v1/github-app/setup", h.setup)
}

func (h *githubAppHandlers) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return "", false
	}
	return claims.OrgID, true
}

func (h *githubAppHandlers) status(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	status, err := h.svc.Status(r.Context(), orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"configured": false})
			return
		}
		h.fail(w, "github_app.status", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "github_app": status})
}

func (h *githubAppHandlers) manifest(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	result, err := h.svc.CreateManifest(r.Context(), orgID)
	if err != nil {
		h.fail(w, "github_app.manifest", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *githubAppHandlers) installation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		InstallationID int64 `json:"installation_id"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	status, err := h.svc.SetInstallation(r.Context(), orgID, req.InstallationID)
	if err != nil {
		h.fail(w, "github_app.installation", err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *githubAppHandlers) setup(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("installation_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		http.Error(w, "invalid installation id", http.StatusBadRequest)
		return
	}
	status, err := h.svc.Setup(r.Context(), r.URL.Query().Get("state"), id)
	if err != nil {
		h.log.Warn("github_app.setup", slog.Any("err", err))
		http.Error(w, "could not complete the installation", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(githubsetup.CallbackPage(status))) //nolint:gosec // escaped in CallbackPage
}

func (h *githubAppHandlers) callback(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.Complete(r.Context(), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		h.log.Warn("github_app.callback", slog.Any("err", err))
		http.Error(w, "could not complete the GitHub App setup", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(githubsetup.CallbackPage(status))) //nolint:gosec // values are escaped by CallbackPage
}

func (h *githubAppHandlers) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "github app operation failed")
	}
}

var _ = strconv.Itoa
