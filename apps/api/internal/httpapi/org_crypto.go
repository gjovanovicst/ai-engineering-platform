package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgcrypto"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// OrgCryptoService is the BYOK configuration surface.
type OrgCryptoService interface {
	Get(ctx context.Context, orgID string) (orgcrypto.Config, error)
	Set(ctx context.Context, orgID, actorID, kmsURL, keyID, token string) (orgcrypto.Config, error)
	Disable(ctx context.Context, orgID, actorID string) error
}

type orgCryptoHandlers struct {
	svc    OrgCryptoService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *orgCryptoHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/crypto", h.get)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/crypto", h.set)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/crypto", h.disable)
}

func (h *orgCryptoHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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

func orgCryptoJSON(cfg orgcrypto.Config) map[string]any {
	return map[string]any{
		"kms_url": cfg.KMSURL, "key_id": cfg.KeyID, "status": cfg.Status,
		"updated_at": cfg.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *orgCryptoHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	cfg, err := h.svc.Get(r.Context(), claims.OrgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "no BYOK configuration")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load BYOK")
		return
	}
	writeJSON(w, http.StatusOK, orgCryptoJSON(cfg))
}

func (h *orgCryptoHandlers) set(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		KMSURL string `json:"kms_url"`
		KeyID  string `json:"key_id"`
		Token  string `json:"token"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	cfg, err := h.svc.Set(r.Context(), claims.OrgID, claims.Subject, req.KMSURL, req.KeyID, req.Token)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("orgcrypto.set", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save BYOK")
		return
	}
	writeJSON(w, http.StatusOK, orgCryptoJSON(cfg))
}

func (h *orgCryptoHandlers) disable(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if err := h.svc.Disable(r.Context(), claims.OrgID, claims.Subject); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not disable BYOK")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
