package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/previews"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// PreviewService is the preview environment use case.
type PreviewService interface {
	Create(ctx context.Context, orgID, projectID, name, image string, port int, ttl time.Duration) (previews.Environment, error)
	List(ctx context.Context, orgID, projectID string) ([]previews.Environment, error)
	RequestDestroy(ctx context.Context, orgID, id string) error
}

type previewHandlers struct {
	svc    PreviewService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newPreviewHandlers(svc PreviewService, issuer *auth.Issuer, log *slog.Logger) *previewHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &previewHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *previewHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/previews", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/previews", h.create)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/projects/{project}/previews/{preview}", h.destroy)
}

func (h *previewHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (string, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return "", false
	}
	return claims.OrgID, true
}

func previewJSON(env previews.Environment) map[string]any {
	return map[string]any{
		"id": env.ID, "name": env.Name, "image": env.Image, "port": env.Port,
		"status": env.Status, "url": env.URL, "error": env.Error,
		"expires_at": env.ExpiresAt, "created_at": env.CreatedAt,
	}
}

func (h *previewHandlers) list(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), orgID, r.PathValue("project"))
	if err != nil {
		h.log.Error("previews.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read previews")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, previewJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *previewHandlers) create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		Image      string `json:"image"`
		Port       int    `json:"port"`
		TTLMinutes int    `json:"ttl_minutes"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	env, err := h.svc.Create(r.Context(), orgID, r.PathValue("project"), req.Name, req.Image, req.Port, time.Duration(req.TTLMinutes)*time.Minute)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("previews.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the preview")
		return
	}
	writeJSON(w, http.StatusAccepted, previewJSON(env))
}

func (h *previewHandlers) destroy(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	if err := h.svc.RequestDestroy(r.Context(), orgID, r.PathValue("preview")); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "preview not found")
			return
		}
		h.log.Error("previews.destroy", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not destroy the preview")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
