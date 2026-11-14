package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/sso"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// SSOService is the SSO config surface.
type SSOService interface {
	Get(ctx context.Context, orgID string) (sso.Config, error)
	Set(ctx context.Context, orgID, actorID string, cfg sso.Config) (sso.Config, error)
	Fetch(ctx context.Context, orgID, actorID, metadataURL string) (sso.Config, error)
}

type ssoHandlers struct {
	svc    SSOService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *ssoHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/sso", h.get)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/sso", h.set)
	mux.HandleFunc("POST /api/v1/orgs/{org}/sso/fetch", h.fetch)
}

func (h *ssoHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return nil, false
	}
	return claims, true
}

func ssoJSON(cfg sso.Config) map[string]any {
	return map[string]any{
		"entity_id": cfg.EntityID, "metadata_url": cfg.MetadataURL, "sso_url": cfg.SSOURL,
		"certificate_pem": cfg.CertificatePEM, "enabled": cfg.Enabled,
		"updated_at": cfg.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *ssoHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	cfg, err := h.svc.Get(r.Context(), claims.OrgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "no SSO configuration")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load SSO")
		return
	}
	writeJSON(w, http.StatusOK, ssoJSON(cfg))
}

func (h *ssoHandlers) fetch(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		MetadataURL string `json:"metadata_url"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	cfg, err := h.svc.Fetch(r.Context(), claims.OrgID, claims.Subject, req.MetadataURL)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not fetch metadata")
		return
	}
	writeJSON(w, http.StatusOK, ssoJSON(cfg))
}

func (h *ssoHandlers) set(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		EntityID       string `json:"entity_id"`
		MetadataURL    string `json:"metadata_url"`
		SSOURL         string `json:"sso_url"`
		CertificatePEM string `json:"certificate_pem"`
		Enabled        bool   `json:"enabled"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	cfg, err := h.svc.Set(r.Context(), claims.OrgID, claims.Subject, sso.Config{
		EntityID: req.EntityID, MetadataURL: req.MetadataURL, SSOURL: req.SSOURL, CertificatePEM: req.CertificatePEM, Enabled: req.Enabled,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save SSO")
		return
	}
	writeJSON(w, http.StatusOK, ssoJSON(cfg))
}
