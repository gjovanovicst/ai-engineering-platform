package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/usage"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// UsageStore implements usage.Store.
type UsageStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewUsageStore wires the store.
func NewUsageStore(pool *pgxpool.Pool) *UsageStore {
	return &UsageStore{pool: pool, q: pgstore.New(pool)}
}

// Record inserts a usage row.
func (s *UsageStore) Record(ctx context.Context, e usage.Entry) error {
	var cost pgtype.Numeric
	if err := cost.Scan(strconv.FormatFloat(e.Cost, 'f', 6, 64)); err != nil {
		return err
	}
	return db.WithOrg(ctx, s.pool, e.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.q.WithTx(tx).RecordUsage(ctx, pgstore.RecordUsageParams{
			OrgID: toUUID(e.OrgID), RunID: optionalUUID(e.RunID), Provider: e.Provider, Model: e.Model,
			InputTokens: int32(e.Input), OutputTokens: int32(e.Output), CostUsd: cost, //nolint:gosec // token counts
		})
		return err
	})
}

// Price returns the stored price for a provider/model pair.
func (s *UsageStore) Price(ctx context.Context, orgID, provider, model string) (usage.Price, bool, error) {
	var price usage.Price
	found := false
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetModelPrice(ctx, pgstore.GetModelPriceParams{
			OrgID: toUUID(orgID), Provider: provider, Model: model,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = true
		price = priceFromRow(row)
		return nil
	})
	return price, found, err
}

// UpsertPrice stores a price.
func (s *UsageStore) UpsertPrice(ctx context.Context, orgID string, p usage.Price) (usage.Price, error) {
	var out usage.Price
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		in, err := numericFromFloat(p.InputPer1K)
		if err != nil {
			return err
		}
		outN, err := numericFromFloat(p.OutputPer1K)
		if err != nil {
			return err
		}
		row, err := s.q.WithTx(tx).UpsertModelPrice(ctx, pgstore.UpsertModelPriceParams{
			OrgID: toUUID(orgID), Provider: p.Provider, Model: p.Model,
			InputPer1k: in, OutputPer1k: outN,
		})
		if err != nil {
			return err
		}
		out = priceFromRow(row)
		return nil
	})
	return out, err
}

// ListPrices returns the price table.
func (s *UsageStore) ListPrices(ctx context.Context, orgID string) ([]usage.Price, error) {
	var out []usage.Price
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListModelPrices(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		out = make([]usage.Price, 0, len(rows))
		for _, row := range rows {
			out = append(out, priceFromRow(row))
		}
		return nil
	})
	return out, err
}

func priceFromRow(row pgstore.ModelPrice) usage.Price {
	in, _ := row.InputPer1k.Float64Value()
	out, _ := row.OutputPer1k.Float64Value()
	return usage.Price{
		Provider: row.Provider, Model: row.Model,
		InputPer1K: in.Float64, OutputPer1K: out.Float64,
	}
}

func numericFromFloat(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', 6, 64))
	return n, err
}

// Cost sums the cost in the window.
func (s *UsageStore) Cost(ctx context.Context, orgID string, days int) (float64, int, error) {
	var cost float64
	var rows int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UsageCost(ctx, pgstore.UsageCostParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped
		})
		if err != nil {
			return err
		}
		cost, rows = row.Cost, int(row.Rows)
		return nil
	})
	return cost, rows, err
}

// ListForRun returns the usage rows of one run.
func (s *UsageStore) ListForRun(ctx context.Context, orgID, runID string) ([]usage.Entry, error) {
	var out []usage.Entry
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListUsageForRun(ctx, pgstore.ListUsageForRunParams{
			OrgID: toUUID(orgID), RunID: toUUID(runID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, usage.Entry{
				OrgID: uuidString(row.OrgID), RunID: uuidString(row.RunID),
				Provider: row.Provider, Model: row.Model,
				Input: int(row.InputTokens), Output: int(row.OutputTokens), Cost: numericFloat(row.CostUsd),
			})
		}
		return nil
	})
	return out, err
}

func numericFloat(n pgtype.Numeric) float64 {
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}
