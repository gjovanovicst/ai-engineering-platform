package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// SearchService finds tasks.
type SearchService interface {
	Search(ctx context.Context, orgID, projectID, query string, limit int) ([]domain.Task, error)
}

type searchHandlers struct {
	svc    SearchService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newSearchHandlers(svc SearchService, issuer *auth.Issuer, log *slog.Logger) *searchHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &searchHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *searchHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/search", h.search)
}

func (h *searchHandlers) search(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	items, err := h.svc.Search(r.Context(), claims.OrgID, r.URL.Query().Get("project"), r.URL.Query().Get("q"), 20)
	if err != nil {
		h.log.Error("search.tasks", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "search failed")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, task := range items {
		data = append(data, map[string]any{
			"type": "task", "id": task.ID, "title": task.Title,
			"subtitle": task.Ref + " · " + string(task.Status),
			"url":      "/projects/" + task.ProjectID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
