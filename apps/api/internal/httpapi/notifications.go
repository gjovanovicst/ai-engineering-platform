package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/notifications"
)

// NotificationService lists and reads the current user's notifications.
type NotificationService interface {
	List(ctx context.Context, orgID, userID string, unreadOnly bool, limit int) ([]notifications.Notification, error)
	MarkRead(ctx context.Context, orgID, userID string, ids []string) error
}

type notificationHandlers struct {
	svc    NotificationService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newNotificationHandlers(svc NotificationService, issuer *auth.Issuer, log *slog.Logger) *notificationHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &notificationHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *notificationHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/notifications", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/notifications/read", h.read)
}

func (h *notificationHandlers) claims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	return claims, true
}

func (h *notificationHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	unreadOnly, _ := strconv.ParseBool(r.URL.Query().Get("unread"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.svc.List(r.Context(), claims.OrgID, claims.Subject, unreadOnly, limit)
	if err != nil {
		h.log.Error("notifications.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list notifications")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var readAt any
		if item.Read {
			readAt = item.CreatedAt.UTC().Format(time.RFC3339)
		}
		data = append(data, map[string]any{
			"id": item.ID, "kind": item.Type, "title": item.Title, "body": item.Body,
			"target": item.Link, "read_at": readAt, "created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *notificationHandlers) read(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(w, r)
	if !ok {
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if err := h.svc.MarkRead(r.Context(), claims.OrgID, claims.Subject, req.IDs); err != nil {
		h.log.Error("notifications.read", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not mark the notifications")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
