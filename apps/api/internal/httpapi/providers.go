package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/providers"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// ProviderService stores AI provider configuration.
type ProviderService interface {
	Create(ctx context.Context, in providers.CreateInput) (providers.Provider, error)
	List(ctx context.Context, orgID string) ([]providers.Provider, error)
	RotateKey(ctx context.Context, orgID, id, key string) (providers.Provider, error)
	SetEnabled(ctx context.Context, orgID, id string, enabled bool) (providers.Provider, error)
	Delete(ctx context.Context, orgID, id string) error
}

type providerHandlers struct {
	svc    ProviderService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newProviderHandlers(svc ProviderService, issuer *auth.Issuer, log *slog.Logger) *providerHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &providerHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *providerHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/providers", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/providers", h.create)
	mux.HandleFunc("POST /api/v1/orgs/{org}/providers/{provider}/rotate", h.rotate)
	mux.HandleFunc("PATCH /api/v1/orgs/{org}/providers/{provider}", h.setEnabled)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/providers/{provider}", h.remove)
}

func (h *providerHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
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

func (h *providerHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID)
	if err != nil {
		h.log.Error("providers.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list providers")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, provider := range items {
		data = append(data, providerJSON(provider))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *providerHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		Kind    string `json:"kind"`
		APIKey  string `json:"api_key"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	provider, err := h.svc.Create(r.Context(), providers.CreateInput{
		OrgID: claims.OrgID, Name: req.Name, BaseURL: req.BaseURL, Model: req.Model,
		Kind: req.Kind, APIKey: req.APIKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "a provider with this name already exists")
		case errors.Is(err, providers.ErrCryptoUnavailable):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("providers.create", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not save the provider")
		}
		return
	}
	writeJSON(w, http.StatusCreated, providerJSON(provider))
}

func (h *providerHandlers) rotate(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		APIKey string `json:"api_key"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	provider, err := h.svc.RotateKey(r.Context(), claims.OrgID, r.PathValue("provider"), req.APIKey)
	if err != nil {
		h.writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, providerJSON(provider))
}

func (h *providerHandlers) setEnabled(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, 1<<20, &req) || req.Enabled == nil {
		if req.Enabled == nil {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "enabled is required")
		}
		return
	}
	provider, err := h.svc.SetEnabled(r.Context(), claims.OrgID, r.PathValue("provider"), *req.Enabled)
	if err != nil {
		h.writeProviderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, providerJSON(provider))
}

func (h *providerHandlers) remove(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), claims.OrgID, r.PathValue("provider")); err != nil {
		h.writeProviderError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *providerHandlers) writeProviderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "provider not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", "a provider with this name already exists")
	case errors.Is(err, providers.ErrCryptoUnavailable):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		h.log.Error("providers", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "provider operation failed")
	}
}

func providerJSON(p providers.Provider) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "base_url": p.BaseURL, "model": p.Model,
		"has_key": p.HasKey, "key_hint": p.KeyHint, "enabled": p.Enabled,
		"created_at": p.CreatedAt.UTC().Format(time.RFC3339),
	}
}
