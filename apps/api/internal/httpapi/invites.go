package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/invites"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// InviteService creates, lists and accepts invites.
type InviteService interface {
	Create(ctx context.Context, in invites.CreateInput) (invites.Created, error)
	List(ctx context.Context, orgID string, limit int) ([]invites.Invite, error)
	Accept(ctx context.Context, orgID, token, userID string) (invites.Invite, error)
	RegisterAccept(ctx context.Context, token, name, password string) (auth.TokenPair, error)
}

type inviteHandlers struct {
	svc    InviteService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newInviteHandlers(svc InviteService, issuer *auth.Issuer, log *slog.Logger) *inviteHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &inviteHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *inviteHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/invites", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/invites", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/invites/accept", h.accept)
	mux.HandleFunc("POST /api/v1/invites/accept", h.registerAccept)
}

func (h *inviteHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "only admins can invite")
		return
	}
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	created, err := h.svc.Create(r.Context(), invites.CreateInput{
		OrgID: claims.OrgID, Email: req.Email, Role: req.Role, CreatedBy: claims.Subject,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("invites.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the invite")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": created.Invite.ID, "email": created.Invite.Email, "role": created.Invite.Role,
		"expires_at": created.Invite.ExpiresAt.UTC().Format(time.RFC3339),
		"token":      created.Token,
	})
}

func (h *inviteHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "only admins can list invites")
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID, 50)
	if err != nil {
		h.log.Error("invites.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list invites")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, inv := range items {
		item := map[string]any{
			"id": inv.ID, "email": inv.Email, "role": inv.Role,
			"expires_at": inv.ExpiresAt.UTC().Format(time.RFC3339),
		}
		if inv.AcceptedAt != nil {
			item["accepted_at"] = inv.AcceptedAt.UTC().Format(time.RFC3339)
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// registerAccept accepts an invite for a user who has no account yet.
func (h *inviteHandlers) registerAccept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	pair, err := h.svc.RegisterAccept(r.Context(), req.Token, req.Name, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "invite not found")
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "the invite was already accepted")
		case errors.Is(err, domain.ErrValidation), errors.Is(err, auth.ErrWeakPassword):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("invites.register_accept", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not accept the invite")
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"access_token": pair.AccessToken, "refresh_token": pair.RefreshToken,
		"expires_in": pair.ExpiresIn, "refresh_expires_in": pair.RefreshExpiresIn,
	})
}

func (h *inviteHandlers) accept(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	invite, err := h.svc.Accept(r.Context(), r.PathValue("org"), req.Token, claims.Subject)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "invite not found")
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "the invite was already accepted")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("invites.accept", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not accept the invite")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"org_id": invite.OrgID, "email": invite.Email, "role": invite.Role,
	})
}
