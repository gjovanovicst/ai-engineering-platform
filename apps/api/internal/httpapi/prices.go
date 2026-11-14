package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/usage"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// PriceService stores model prices.
type PriceService interface {
	UpsertPrice(ctx context.Context, orgID string, p usage.Price) (usage.Price, error)
	ListPrices(ctx context.Context, orgID string) ([]usage.Price, error)
}

type priceHandlers struct {
	svc    PriceService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newPriceHandlers(svc PriceService, issuer *auth.Issuer, log *slog.Logger) *priceHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &priceHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *priceHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/model-prices", h.list)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/model-prices", h.upsert)
}

func (h *priceHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	return claims, true
}

func (h *priceHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListPrices(r.Context(), claims.OrgID)
	if err != nil {
		h.log.Error("prices.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list prices")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, price := range items {
		data = append(data, priceJSON(price))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *priceHandlers) upsert(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Provider    string  `json:"provider"`
		Model       string  `json:"model"`
		InputPer1K  float64 `json:"input_per_1k"`
		OutputPer1K float64 `json:"output_per_1k"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	price, err := h.svc.UpsertPrice(r.Context(), claims.OrgID, usage.Price{
		Provider: req.Provider, Model: req.Model,
		InputPer1K: req.InputPer1K, OutputPer1K: req.OutputPer1K,
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("prices.upsert", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the price")
		return
	}
	writeJSON(w, http.StatusOK, priceJSON(price))
}

func priceJSON(p usage.Price) map[string]any {
	return map[string]any{
		"provider": p.Provider, "model": p.Model,
		"input_per_1k": p.InputPer1K, "output_per_1k": p.OutputPer1K,
	}
}
