package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/restores"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RestoreService queues PITR restores and exposes DR health.
type RestoreService interface {
	Create(ctx context.Context, orgID, actorID string, target time.Time) (restores.Request, error)
	List(ctx context.Context, orgID string) ([]restores.Request, error)
	DR(ctx context.Context, orgID string) (restores.DR, error)
}

type restoreHandlers struct {
	svc    RestoreService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *restoreHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/restores", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/restores", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/dr", h.dr)
}

func (h *restoreHandlers) authorize(w http.ResponseWriter, r *http.Request, role string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, role) {
		writeProblem(w, http.StatusForbidden, "forbidden", role+" role is required")
		return nil, false
	}
	return claims, true
}

func (h *restoreHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list restores")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id": item.ID, "target_time": item.TargetTime.UTC().Format(time.RFC3339),
			"status": item.Status, "detail": item.Detail,
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *restoreHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		TargetTime string `json:"target_time"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	target, err := time.Parse(time.RFC3339, req.TargetTime)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "target_time must be RFC3339")
		return
	}
	item, err := h.svc.Create(r.Context(), claims.OrgID, claims.Subject, target)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("restores.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not queue the restore")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": item.ID, "status": item.Status})
}

func (h *restoreHandlers) dr(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	dr, err := h.svc.DR(r.Context(), claims.OrgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read DR status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"wal_failed": dr.WALFailed, "wal_archived": dr.WALArchived,
		"wal_age_seconds": dr.WALAge, "base_age_seconds": dr.BaseAge,
		"base_backup":         dr.BaseName,
		"wal_ok":              dr.WALFailed == 0 || dr.WALAge < 900,
		"base_ok":             dr.BaseAge > 0 && dr.BaseAge < 172800,
		"offsite_age_seconds": dr.OffsiteAge,
		"restore_succeeded":   dr.RestoreSucceeded, "restore_failed": dr.RestoreFailed,
		"last_restore_age_seconds": dr.LastRestoreAge, "avg_restore_seconds": dr.AvgRestoreSeconds,
	})
}
