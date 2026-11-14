package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/analytics"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// AnalyticsService computes org metrics.
type AnalyticsService interface {
	Summary(ctx context.Context, orgID string, days int) (analytics.Summary, error)
}

type analyticsHandlers struct {
	svc    AnalyticsService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newAnalyticsHandlers(svc AnalyticsService, issuer *auth.Issuer, log *slog.Logger) *analyticsHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &analyticsHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *analyticsHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/metrics", h.summary)
}

func (h *analyticsHandlers) summary(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			days = n
		}
	}
	summary, err := h.svc.Summary(r.Context(), claims.OrgID, days)
	if err != nil {
		h.log.Error("analytics.summary", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not compute metrics")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
