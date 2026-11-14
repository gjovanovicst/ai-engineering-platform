package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/designstandards"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// DesignStandardService versions the org design standard.
type DesignStandardService interface {
	List(ctx context.Context, orgID string, limit int) ([]designstandards.Standard, error)
	Get(ctx context.Context, orgID, id string) (designstandards.Standard, error)
	Create(ctx context.Context, orgID, actorID, name string, spec []byte) (designstandards.Standard, error)
	CreateVersion(ctx context.Context, orgID, actorID, id string, spec []byte) (designstandards.Standard, error)
	Publish(ctx context.Context, orgID, actorID, id string) (designstandards.Standard, error)
}

type designStandardHandlers struct {
	svc    DesignStandardService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newDesignStandardHandlers(svc DesignStandardService, issuer *auth.Issuer, log *slog.Logger) *designStandardHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &designStandardHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *designStandardHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/design-standards", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/design-standards", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/design-standards/{standard}", h.get)
	mux.HandleFunc("POST /api/v1/orgs/{org}/design-standards/{standard}/versions", h.createVersion)
	mux.HandleFunc("POST /api/v1/orgs/{org}/design-standards/{standard}/publish", h.publish)
}

func (h *designStandardHandlers) claims(w http.ResponseWriter, r *http.Request, role string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, role) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	return claims, true
}

func (h *designStandardHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID, 100)
	if err != nil {
		h.fail(w, "design_standards.list", err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, designStandardJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *designStandardHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, "viewer")
	if !ok {
		return
	}
	item, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("standard"))
	if err != nil {
		h.fail(w, "design_standards.get", err)
		return
	}
	writeJSON(w, http.StatusOK, designStandardJSON(item))
}

func (h *designStandardHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, "designer")
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
	created, err := h.svc.Create(r.Context(), claims.OrgID, claims.Subject, req.Name, req.Spec)
	if err != nil {
		h.fail(w, "design_standards.create", err)
		return
	}
	writeJSON(w, http.StatusCreated, designStandardJSON(created))
}

func (h *designStandardHandlers) createVersion(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, "designer")
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
	created, err := h.svc.CreateVersion(r.Context(), claims.OrgID, claims.Subject, r.PathValue("standard"), req.Spec)
	if err != nil {
		h.fail(w, "design_standards.version", err)
		return
	}
	writeJSON(w, http.StatusCreated, designStandardJSON(created))
}

func (h *designStandardHandlers) publish(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r, "designer")
	if !ok {
		return
	}
	published, err := h.svc.Publish(r.Context(), claims.OrgID, claims.Subject, r.PathValue("standard"))
	if err != nil {
		h.fail(w, "design_standards.publish", err)
		return
	}
	writeJSON(w, http.StatusOK, designStandardJSON(published))
}

func (h *designStandardHandlers) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "design standard not found")
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "design standard operation failed")
	}
}

func designStandardJSON(item designstandards.Standard) map[string]any {
	out := map[string]any{
		"id": item.ID, "name": item.Name, "version": item.Version,
		"status": item.Status, "spec": item.Spec, "sha256": item.SHA256,
		"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
	}
	if item.PublishedAt != nil {
		out["published_at"] = item.PublishedAt.UTC().Format(time.RFC3339)
	}
	return out
}
