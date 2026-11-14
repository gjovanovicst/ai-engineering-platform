package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/blueprints"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// BlueprintService validates and instantiates blueprints.
type BlueprintService interface {
	Create(ctx context.Context, orgID, name string, spec json.RawMessage) (blueprints.Blueprint, error)
	List(ctx context.Context, orgID string, limit int) ([]blueprints.Blueprint, error)
	Instantiate(ctx context.Context, orgID, id string) (domain.Project, int, error)
	Publish(ctx context.Context, orgID, id string) (blueprints.Blueprint, error)
	CreateVersion(ctx context.Context, orgID, id string, spec json.RawMessage) (blueprints.Blueprint, error)
}

type blueprintHandlers struct {
	svc    BlueprintService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newBlueprintHandlers(svc BlueprintService, issuer *auth.Issuer, log *slog.Logger) *blueprintHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &blueprintHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *blueprintHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/blueprints", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/blueprints", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/blueprints/{blueprint}/instantiate", h.instantiate)
	mux.HandleFunc("POST /api/v1/orgs/{org}/blueprints/{blueprint}/publish", h.publish)
	mux.HandleFunc("POST /api/v1/orgs/{org}/blueprints/{blueprint}/versions", h.createVersion)
}

func (h *blueprintHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
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

func (h *blueprintHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Name string          `json:"name"`
		Spec json.RawMessage `json:"spec"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if req.Name == "" || len(req.Spec) == 0 {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "name and spec are required")
		return
	}
	bp, err := h.svc.Create(r.Context(), claims.OrgID, req.Name, req.Spec)
	if err != nil {
		var verr *blueprints.ValidationError
		if errors.As(err, &verr) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"type": "validation_failed", "title": "Invalid blueprint", "status": 400, "problems": verr.Problems,
			})
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			writeProblem(w, http.StatusConflict, "conflict", "a blueprint with this name already exists")
			return
		}
		h.log.Error("blueprints.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the blueprint")
		return
	}
	writeJSON(w, http.StatusCreated, blueprintJSON(bp))
}

func (h *blueprintHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID, 50)
	if err != nil {
		h.log.Error("blueprints.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list blueprints")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, bp := range items {
		data = append(data, blueprintJSON(bp))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *blueprintHandlers) instantiate(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	project, tasks, err := h.svc.Instantiate(r.Context(), claims.OrgID, r.PathValue("blueprint"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "blueprint not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			writeProblem(w, http.StatusConflict, "invalid_transition", "only published blueprints can create a project")
			return
		}
		var verr *blueprints.ValidationError
		if errors.As(err, &verr) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", verr.Error())
			return
		}
		h.log.Error("blueprints.instantiate", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the project")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": project.ID, "name": project.Name, "slug": project.Slug, "status": project.Status,
		"created_tasks": tasks,
	})
}

func (h *blueprintHandlers) publish(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	bp, err := h.svc.Publish(r.Context(), claims.OrgID, r.PathValue("blueprint"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "blueprint not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidTransition) {
			writeProblem(w, http.StatusConflict, "invalid_transition", "the blueprint is already published")
			return
		}
		h.log.Error("blueprints.publish", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not publish the blueprint")
		return
	}
	writeJSON(w, http.StatusOK, blueprintJSON(bp))
}

func blueprintJSON(bp blueprints.Blueprint) map[string]any {
	out := map[string]any{
		"id": bp.ID, "name": bp.Name, "version": bp.Version, "status": bp.Status,
		"spec": bp.Spec, "created_at": bp.CreatedAt.UTC().Format(time.RFC3339),
	}
	if bp.PublishedAt != nil {
		out["published_at"] = bp.PublishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (h *blueprintHandlers) createVersion(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Spec json.RawMessage `json:"spec"`
	}
	if r.ContentLength != 0 {
		if !decode(w, r, 1<<20, &req) {
			return
		}
	}
	version, err := h.svc.CreateVersion(r.Context(), claims.OrgID, r.PathValue("blueprint"), req.Spec)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "blueprint not found")
		default:
			h.log.Error("blueprints.version", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not create the version")
		}
		return
	}
	writeJSON(w, http.StatusCreated, blueprintJSON(version))
}
