package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/scim"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// SCIMService exposes read-only SCIM and token management.
type SCIMService interface {
	CreateToken(ctx context.Context, orgID, name string) (scim.Token, string, error)
	ListTokens(ctx context.Context, orgID string) ([]scim.Token, error)
	RevokeToken(ctx context.Context, orgID, id string) error
	Authenticate(ctx context.Context, raw string) (string, error)
	Users(ctx context.Context, orgID string) ([]scim.User, error)
	CreateUser(ctx context.Context, orgID, email, name string) (scim.User, error)
	DeactivateUser(ctx context.Context, orgID, userID string) error
	SetRole(ctx context.Context, orgID, userID, role string) error
	SetName(ctx context.Context, orgID, userID, name string) error
	SetEmail(ctx context.Context, orgID, userID, email string) error
}

type scimHandlers struct {
	svc    SCIMService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *scimHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/scim-tokens", h.createToken)
	mux.HandleFunc("GET /api/v1/orgs/{org}/scim-tokens", h.listTokens)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/scim-tokens/{token}", h.revokeToken)
	mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", h.config)
	mux.HandleFunc("GET /scim/v2/Users", h.users)
	mux.HandleFunc("POST /scim/v2/Users", h.createUser)
	mux.HandleFunc("PATCH /scim/v2/Users/{user}", h.patchUser)
}

func (h *scimHandlers) admin(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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

func (h *scimHandlers) orgFromBearer(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if raw == "" || raw == r.Header.Get("Authorization") {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "a SCIM bearer token is required")
		return "", false
	}
	orgID, err := h.svc.Authenticate(r.Context(), raw)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "invalid SCIM token")
		return "", false
	}
	return orgID, true
}

func (h *scimHandlers) createToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.admin(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	token, raw, err := h.svc.CreateToken(r.Context(), claims.OrgID, req.Name)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("scim.create_token", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the token")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": token.ID, "name": token.Name, "created_at": token.CreatedAt, "token": raw})
}

func (h *scimHandlers) listTokens(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.admin(w, r)
	if !ok {
		return
	}
	tokens, err := h.svc.ListTokens(r.Context(), claims.OrgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list tokens")
		return
	}
	out := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, map[string]any{"id": token.ID, "name": token.Name, "created_at": token.CreatedAt, "revoked_at": token.RevokedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *scimHandlers) revokeToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.admin(w, r)
	if !ok {
		return
	}
	if err := h.svc.RevokeToken(r.Context(), claims.OrgID, r.PathValue("token")); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not revoke the token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *scimHandlers) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"schemas":               []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri":      "https://github.com/gjovanovicst/ai-engineering-platform",
		"patch":                 map[string]bool{"supported": false},
		"bulk":                  map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":                map[string]any{"supported": true, "maxResults": 200},
		"changePassword":        map[string]bool{"supported": false},
		"sort":                  map[string]bool{"supported": false},
		"etag":                  map[string]bool{"supported": false},
		"authenticationSchemes": []map[string]any{{"type": "oauthbearertoken", "name": "Bearer", "primary": true}},
	})
}

func (h *scimHandlers) createUser(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgFromBearer(w, r)
	if !ok {
		return
	}
	var req struct {
		UserName string `json:"userName"`
		Name     struct {
			Formatted string `json:"formatted"`
		} `json:"name"`
		Emails []struct {
			Value string `json:"value"`
		} `json:"emails"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	email := req.UserName
	if email == "" && len(req.Emails) > 0 {
		email = req.Emails[0].Value
	}
	user, err := h.svc.CreateUser(r.Context(), orgID, email, req.Name.Formatted)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "invalidValue", err.Error())
			return
		}
		h.log.Error("scim.create_user", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not provision the user")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"schemas":  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
		"id":       user.ID,
		"userName": user.Email,
		"name":     map[string]string{"formatted": user.Name},
		"emails":   []map[string]any{{"value": user.Email, "primary": true}},
		"active":   user.Active,
		"meta":     map[string]any{"resourceType": "User", "created": user.CreatedAt.UTC().Format(time.RFC3339), "location": "/scim/v2/Users/" + user.ID},
	})
}

func (h *scimHandlers) patchUser(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgFromBearer(w, r)
	if !ok {
		return
	}
	var req struct {
		Operations []struct {
			Op    string `json:"op"`
			Path  string `json:"path"`
			Value any    `json:"value"`
		} `json:"Operations"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if len(req.Operations) != 1 {
		writeProblem(w, http.StatusBadRequest, "invalidValue", "exactly one operation is required")
		return
	}
	op := req.Operations[0]
	if !strings.EqualFold(op.Op, "replace") {
		writeProblem(w, http.StatusBadRequest, "invalidValue", "only replace is supported")
		return
	}
	userID := r.PathValue("user")
	var err error
	switch op.Path {
	case "active":
		active, isBool := op.Value.(bool)
		if !isBool || active {
			writeProblem(w, http.StatusBadRequest, "invalidValue", "only active=false is supported")
			return
		}
		err = h.svc.DeactivateUser(r.Context(), orgID, userID)
	case "name.formatted":
		name, isString := op.Value.(string)
		if !isString {
			writeProblem(w, http.StatusBadRequest, "invalidValue", "name.formatted must be a string")
			return
		}
		err = h.svc.SetName(r.Context(), orgID, userID, name)
	case "emails":
		email := scimEmail(op.Value)
		if email == "" {
			writeProblem(w, http.StatusBadRequest, "invalidValue", "emails value is required")
			return
		}
		err = h.svc.SetEmail(r.Context(), orgID, userID, email)
	case "roles":
		role := scimRole(op.Value)
		if role == "" {
			writeProblem(w, http.StatusBadRequest, "invalidValue", "roles value is required")
			return
		}
		err = h.svc.SetRole(r.Context(), orgID, userID, role)
	default:
		writeProblem(w, http.StatusBadRequest, "invalidValue", "only active, roles, name.formatted or emails are supported")
		return
	}
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "invalidValue", err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "membership not found")
			return
		}
		h.log.Error("scim.patch", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not update the user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scimEmail(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return scimEmail(typed["value"])
	case []any:
		if len(typed) > 0 {
			return scimEmail(typed[0])
		}
	}
	return ""
}

// scimRole accepts "admin", {"value":"admin"} or [{"value":"admin"}].
func scimRole(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case map[string]any:
		return scimRole(typed["value"])
	case []any:
		if len(typed) > 0 {
			return scimRole(typed[0])
		}
	}
	return ""
}

func (h *scimHandlers) users(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgFromBearer(w, r)
	if !ok {
		return
	}
	users, err := h.svc.Users(r.Context(), orgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list users")
		return
	}
	resources := make([]map[string]any, 0, len(users))
	for _, user := range users {
		resources = append(resources, map[string]any{
			"schemas":  []string{"urn:ietf:params:scim:schemas:core:2.0:User"},
			"id":       user.ID,
			"userName": user.Email,
			"name":     map[string]string{"formatted": user.Name},
			"emails":   []map[string]any{{"value": user.Email, "primary": true}},
			"active":   true,
			"meta":     map[string]any{"resourceType": "User", "created": user.CreatedAt.UTC().Format(time.RFC3339)},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": len(resources),
		"startIndex":   1,
		"itemsPerPage": len(resources),
		"Resources":    resources,
	})
}
