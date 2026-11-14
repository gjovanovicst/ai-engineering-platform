package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/providers"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// ProviderStore implements providers.Store.
type ProviderStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewProviderStore wires the store.
func NewProviderStore(pool *pgxpool.Pool) *ProviderStore {
	return &ProviderStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts a provider config.
func (s *ProviderStore) Create(ctx context.Context, p providers.Stored) (providers.Stored, error) {
	var out providers.Stored
	err := db.WithOrg(ctx, s.pool, p.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateProvider(ctx, pgstore.CreateProviderParams{
			OrgID: toUUID(p.OrgID), Name: p.Name, BaseUrl: p.BaseURL, Model: p.Model,
			ApiKeyEnc: p.APIKeyEnc, KeyHint: nilIfEmpty(p.KeyHint), Kind: p.Kind,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = providerFromRow(row)
		return nil
	})
	return out, err
}

// List returns the org providers.
func (s *ProviderStore) List(ctx context.Context, orgID string) ([]providers.Stored, error) {
	var out []providers.Stored
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListProviders(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		out = make([]providers.Stored, 0, len(rows))
		for _, row := range rows {
			out = append(out, providerFromRow(row))
		}
		return nil
	})
	return out, err
}

// Get returns one provider.
func (s *ProviderStore) Get(ctx context.Context, orgID, id string) (providers.Stored, error) {
	var out providers.Stored
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetProvider(ctx, pgstore.GetProviderParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = providerFromRow(row)
		return nil
	})
	return out, err
}

// Default returns the first enabled provider.
func (s *ProviderStore) Default(ctx context.Context, orgID string) (providers.Stored, error) {
	var out providers.Stored
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).DefaultProvider(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = providerFromRow(row)
		return nil
	})
	return out, err
}

func providerFromRow(row pgstore.AiProviderConfig) providers.Stored {
	p := providers.Provider{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), Name: row.Name,
		BaseURL: row.BaseUrl, Model: row.Model, Kind: row.Kind, Enabled: row.Enabled,
		CreatedAt: row.CreatedAt.Time, HasKey: len(row.ApiKeyEnc) > 0,
	}
	if row.KeyHint != nil {
		p.KeyHint = *row.KeyHint
	}
	return providers.Stored{Provider: p, APIKeyEnc: row.ApiKeyEnc}
}

// RotateKey replaces the encrypted key.
func (s *ProviderStore) RotateKey(ctx context.Context, orgID, id string, enc []byte, hint string) (providers.Stored, error) {
	var out providers.Stored
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).RotateProviderKey(ctx, pgstore.RotateProviderKeyParams{
			ID: toUUID(id), OrgID: toUUID(orgID), ApiKeyEnc: enc, KeyHint: &hint,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = providerFromRow(row)
		return nil
	})
	return out, err
}

// SetEnabled toggles a provider.
func (s *ProviderStore) SetEnabled(ctx context.Context, orgID, id string, enabled bool) (providers.Stored, error) {
	var out providers.Stored
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProviderEnabled(ctx, pgstore.SetProviderEnabledParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Enabled: enabled,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = providerFromRow(row)
		return nil
	})
	return out, err
}

// Delete removes a provider.
func (s *ProviderStore) Delete(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteProvider(ctx, pgstore.DeleteProviderParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
	})
}
