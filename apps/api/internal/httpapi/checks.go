package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
)

// CheckService records and lists CI checks.
type CheckService interface {
	Record(ctx context.Context, orgID, repoID, prID, runID string, c checks.Check) (checks.Check, error)
	Failed(ctx context.Context, orgID, prID string) (bool, error)
	ListForPullRequest(ctx context.Context, orgID, prID string) ([]checks.Check, error)
	ListForRun(ctx context.Context, orgID, runID string) ([]checks.Check, error)
}

type checkHandlers struct {
	svc    CheckService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newCheckHandlers(svc CheckService, issuer *auth.Issuer, log *slog.Logger) *checkHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &checkHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *checkHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/pull-requests/{pr}/checks", h.forPR)
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}/checks", h.forRun)
}

func (h *checkHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	return claims, true
}

func (h *checkHandlers) forPR(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListForPullRequest(r.Context(), claims.OrgID, r.PathValue("pr"))
	if err != nil {
		h.log.Error("checks.list_pr", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list checks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": checkJSON(items)})
}

func (h *checkHandlers) forRun(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListForRun(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		h.log.Error("checks.list_run", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list checks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": checkJSON(items)})
}

func checkJSON(items []checks.Check) []map[string]any {
	data := make([]map[string]any, 0, len(items))
	for _, check := range items {
		data = append(data, map[string]any{
			"id": check.ID, "name": check.Name, "status": check.Status, "output": check.Output,
			"duration_ms": check.DurationMS, "commit_sha": check.CommitSHA, "url": check.URL,
		})
	}
	return data
}
