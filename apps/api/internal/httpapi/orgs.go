package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// OrgService reads and mutates the tenant record.
type OrgService interface {
	Get(ctx context.Context, orgID string) (orgs.Org, error)
	Rename(ctx context.Context, orgID, actorID, name string) (orgs.Org, error)
	Delete(ctx context.Context, orgID, actorID, confirmSlug string) error
	SetLegalHold(ctx context.Context, orgID, actorID string, hold bool) (orgs.Org, error)
	SetRegion(ctx context.Context, orgID, actorID, region string) (orgs.Org, error)
}

type orgHandlers struct {
	svc    OrgService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newOrgHandlers(svc OrgService, issuer *auth.Issuer, log *slog.Logger) *orgHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &orgHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *orgHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}", h.get)
	mux.HandleFunc("PATCH /api/v1/orgs/{org}", h.rename)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}", h.delete)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/legal-hold", h.setLegalHold)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/region", h.setRegion)
}

func (h *orgHandlers) claims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	return claims, true
}

func (h *orgHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	org, err := h.svc.Get(r.Context(), claims.OrgID)
	if err != nil {
		h.fail(w, "orgs.get", err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (h *orgHandlers) rename(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "owner") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	org, err := h.svc.Rename(r.Context(), claims.OrgID, claims.Subject, req.Name)
	if err != nil {
		h.fail(w, "orgs.rename", err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (h *orgHandlers) setLegalHold(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	var req struct {
		Hold bool `json:"hold"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	org, err := h.svc.SetLegalHold(r.Context(), claims.OrgID, claims.Subject, req.Hold)
	if err != nil {
		h.fail(w, "orgs.legal_hold", err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (h *orgHandlers) setRegion(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	var req struct {
		Region string `json:"region"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	org, err := h.svc.SetRegion(r.Context(), claims.OrgID, claims.Subject, req.Region)
	if err != nil {
		h.fail(w, "orgs.region", err)
		return
	}
	writeJSON(w, http.StatusOK, orgJSON(org))
}

func (h *orgHandlers) delete(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "owner") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return
	}
	var req struct {
		ConfirmSlug string `json:"confirm_slug"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if err := h.svc.Delete(r.Context(), claims.OrgID, claims.Subject, req.ConfirmSlug); err != nil {
		h.fail(w, "orgs.delete", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *orgHandlers) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "organisation not found")
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "organisation operation failed")
	}
}

func orgJSON(org orgs.Org) map[string]any {
	return map[string]any{
		"id": org.ID, "slug": org.Slug, "name": org.Name,
		"max_concurrent_runs": org.MaxConcurrentRuns, "legal_hold": org.LegalHold, "region": org.Region,
		"created_at": org.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": org.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
