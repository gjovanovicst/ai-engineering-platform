package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/apikeys"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type apiKeyContextKey struct{}

// APIKeyService issues and resolves service-account keys.
type APIKeyService interface {
	Create(ctx context.Context, orgID, name, role string, ttl time.Duration) (apikeys.Key, string, error)
	List(ctx context.Context, orgID string) ([]apikeys.Key, error)
	Revoke(ctx context.Context, orgID, id string) error
	Authenticate(ctx context.Context, token string) (apikeys.Key, error)
}

// withAPIKeyAuth turns aip.* bearer tokens into claims for the handlers.
func withAPIKeyAuth(next http.Handler, svc APIKeyService) http.Handler {
	if svc == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r)
		if !ok || !strings.HasPrefix(raw, "aip.") {
			next.ServeHTTP(w, r)
			return
		}
		key, err := svc.Authenticate(r.Context(), raw)
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "unauthenticated", "invalid API key")
			return
		}
		claims := &auth.Claims{OrgID: key.OrgID, Role: key.Role, SessionID: "api-key", Scopes: append([]string{"api_key"}, key.Scopes...)}
		claims.Subject = "api-key:" + key.ID
		ctx := context.WithValue(r.Context(), apiKeyContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func claimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(apiKeyContextKey{}).(*auth.Claims)
	return claims, ok
}

type apiKeyHandlers struct {
	svc    APIKeyService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newAPIKeyHandlers(svc APIKeyService, issuer *auth.Issuer, log *slog.Logger) *apiKeyHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &apiKeyHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *apiKeyHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/api-keys", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/api-keys", h.create)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/api-keys/{key}", h.revoke)
}

func (h *apiKeyHandlers) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return "", false
	}
	return claims.OrgID, true
}

func (h *apiKeyHandlers) list(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	keys, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read API keys")
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		item := map[string]any{"id": key.ID, "name": key.Name, "role": key.Role, "created_at": key.CreatedAt}
		if key.RevokedAt != nil {
			item["revoked_at"] = key.RevokedAt
		}
		if key.LastUsedAt != nil {
			item["last_used_at"] = key.LastUsedAt
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *apiKeyHandlers) create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Role        string `json:"role"`
		ExpiresDays int    `json:"expires_in_days"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	key, token, err := h.svc.Create(r.Context(), orgID, req.Name, req.Role, time.Duration(req.ExpiresDays)*24*time.Hour)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the API key")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": key.ID, "name": key.Name, "role": key.Role, "created_at": key.CreatedAt.UTC().Format(time.RFC3339),
		"token": token, "expires_at": key.ExpiresAt,
	})
}

func (h *apiKeyHandlers) revoke(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), orgID, r.PathValue("key")); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not revoke the API key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
