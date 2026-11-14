// Package usage records LLM token usage and cost per run.
package usage

import (
	"context"
	"fmt"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Entry is one usage record.
type Entry struct {
	OrgID    string
	RunID    string
	Provider string
	Model    string
	Input    int
	Output   int
	Cost     float64
}

// Price is the cost of 1K tokens for a provider/model pair.
type Price struct {
	Provider    string
	Model       string
	InputPer1K  float64
	OutputPer1K float64
}

// Store persists usage and prices.
type Store interface {
	Record(ctx context.Context, e Entry) error
	Cost(ctx context.Context, orgID string, days int) (cost float64, rows int, err error)
	Price(ctx context.Context, orgID, provider, model string) (Price, bool, error)
	UpsertPrice(ctx context.Context, orgID string, p Price) (Price, error)
	ListPrices(ctx context.Context, orgID string) ([]Price, error)
	ListForRun(ctx context.Context, orgID, runID string) ([]Entry, error)
}

// Service computes cost from token counts.
type Service struct {
	store       Store
	inputPrice  float64 // USD per 1K input tokens
	outputPrice float64
}

// New wires the service; prices come from configuration.
func New(store Store, inputPer1K, outputPer1K float64) *Service {
	return &Service{store: store, inputPrice: inputPer1K, outputPrice: outputPer1K}
}

// Record stores a run's usage with the computed cost. A row in model_prices
// wins over the configured default price.
func (s *Service) Record(ctx context.Context, run domain.Run, result agent.Result) error {
	if result.InputTokens == 0 && result.OutputTokens == 0 {
		return nil
	}
	inputPrice, outputPrice := s.inputPrice, s.outputPrice
	if price, ok, err := s.store.Price(ctx, run.OrgID, result.Provider, result.Model); err == nil && ok {
		inputPrice, outputPrice = price.InputPer1K, price.OutputPer1K
	}
	cost := (float64(result.InputTokens)*inputPrice + float64(result.OutputTokens)*outputPrice) / 1000
	return s.store.Record(ctx, Entry{
		OrgID: run.OrgID, RunID: run.ID, Provider: result.Provider, Model: result.Model,
		Input: result.InputTokens, Output: result.OutputTokens, Cost: cost,
	})
}

// UpsertPrice stores the price for a provider/model pair.
func (s *Service) UpsertPrice(ctx context.Context, orgID string, p Price) (Price, error) {
	if p.Provider == "" || p.Model == "" {
		return Price{}, fmt.Errorf("%w: provider and model are required", domain.ErrValidation)
	}
	if p.InputPer1K < 0 || p.OutputPer1K < 0 {
		return Price{}, fmt.Errorf("%w: prices cannot be negative", domain.ErrValidation)
	}
	return s.store.UpsertPrice(ctx, orgID, p)
}

// ListPrices returns the org price table.
func (s *Service) ListPrices(ctx context.Context, orgID string) ([]Price, error) {
	return s.store.ListPrices(ctx, orgID)
}

// ForRun returns the usage rows of one run.
func (s *Service) ForRun(ctx context.Context, orgID, runID string) ([]Entry, error) {
	return s.store.ListForRun(ctx, orgID, runID)
}
