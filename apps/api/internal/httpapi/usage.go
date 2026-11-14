package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/usage"
)

// UsageService reports token usage per run.
type UsageService interface {
	ForRun(ctx context.Context, orgID, runID string) ([]usage.Entry, error)
}

type usageHandlers struct {
	svc    UsageService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newUsageHandlers(svc UsageService, issuer *auth.Issuer, log *slog.Logger) *usageHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &usageHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *usageHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}/usage", h.forRun)
}

func (h *usageHandlers) forRun(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	entries, err := h.svc.ForRun(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		h.log.Error("usage.for_run", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load usage")
		return
	}
	var input, output int
	var cost float64
	data := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		input += e.Input
		output += e.Output
		cost += e.Cost
		data = append(data, map[string]any{
			"provider": e.Provider, "model": e.Model,
			"input_tokens": e.Input, "output_tokens": e.Output, "cost_usd": e.Cost,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"totals": map[string]any{
			"input_tokens": input, "output_tokens": output, "cost_usd": cost,
		},
	})
}
