package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/incidents"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// IncidentService is the incident use-case surface.
type IncidentService interface {
	Create(ctx context.Context, orgID, projectID, title, severity, userID string) (incidents.Incident, error)
	List(ctx context.Context, orgID string, limit int) ([]incidents.Incident, error)
	Resolve(ctx context.Context, orgID, id, userID string) (incidents.Incident, error)
}

type incidentHandlers struct {
	svc    IncidentService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *incidentHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/incidents", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/incidents", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/incidents/{incident}/resolve", h.resolve)
}

func (h *incidentHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	return claims, true
}

func incidentJSON(i incidents.Incident) map[string]any {
	out := map[string]any{
		"id": i.ID, "project_id": i.ProjectID, "title": i.Title, "severity": i.Severity,
		"status": i.Status, "opened_at": i.OpenedAt.UTC().Format(time.RFC3339),
	}
	if i.ResolvedAt != nil {
		out["resolved_at"] = i.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (h *incidentHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		ProjectID string `json:"project_id"`
		Title     string `json:"title"`
		Severity  string `json:"severity"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	incident, err := h.svc.Create(r.Context(), claims.OrgID, req.ProjectID, req.Title, req.Severity, claims.Subject)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("incidents.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the incident")
		return
	}
	writeJSON(w, http.StatusCreated, incidentJSON(incident))
}

func (h *incidentHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID, 100)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list incidents")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, incident := range items {
		out = append(out, incidentJSON(incident))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *incidentHandlers) resolve(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	incident, err := h.svc.Resolve(r.Context(), claims.OrgID, r.PathValue("incident"), claims.Subject)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "incident not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not resolve the incident")
		return
	}
	writeJSON(w, http.StatusOK, incidentJSON(incident))
}
