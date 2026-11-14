package postgres

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/designstandards"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// DesignStandardStore implements designstandards.Store.
type DesignStandardStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewDesignStandardStore wires the store.
func NewDesignStandardStore(pool *pgxpool.Pool) *DesignStandardStore {
	return &DesignStandardStore{pool: pool, q: pgstore.New(pool)}
}

// Create stores one version.
func (s *DesignStandardStore) Create(ctx context.Context, orgID, name string, version int, spec []byte, sha []byte) (designstandards.Standard, error) {
	var out designstandards.Standard
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateDesignStandard(ctx, pgstore.CreateDesignStandardParams{
			OrgID: toUUID(orgID), Name: name, Version: int32(version), Spec: spec, Sha256: sha, //nolint:gosec // small versions
		})
		if err != nil {
			return err
		}
		out = designStandardFromRow(row)
		return nil
	})
	return out, err
}

// NextVersion returns the next version number for a name.
func (s *DesignStandardStore) NextVersion(ctx context.Context, orgID, name string) (int, error) {
	var out int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).NextDesignStandardVersion(ctx, pgstore.NextDesignStandardVersionParams{OrgID: toUUID(orgID), Name: name})
		out = int(n)
		return err
	})
	return out, err
}

// List returns the org's standards.
func (s *DesignStandardStore) List(ctx context.Context, orgID string, limit int) ([]designstandards.Standard, error) {
	var out []designstandards.Standard
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListDesignStandards(ctx, pgstore.ListDesignStandardsParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, designStandardFromRow(row))
		}
		return nil
	})
	return out, err
}

// Get returns one standard.
func (s *DesignStandardStore) Get(ctx context.Context, orgID, id string) (designstandards.Standard, error) {
	var out designstandards.Standard
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetDesignStandard(ctx, pgstore.GetDesignStandardParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = designStandardFromRow(row)
		return nil
	})
	return out, err
}

// Publish freezes a draft.
func (s *DesignStandardStore) Publish(ctx context.Context, orgID, id string) (designstandards.Standard, error) {
	var out designstandards.Standard
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).PublishDesignStandard(ctx, pgstore.PublishDesignStandardParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = designStandardFromRow(row)
		return nil
	})
	return out, err
}

// Audit appends an audit entry.
func (s *DesignStandardStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordDesignAudit(ctx, pgstore.RecordDesignAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}

func designStandardFromRow(row pgstore.DesignStandard) designstandards.Standard {
	out := designstandards.Standard{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), Name: row.Name,
		Version: int(row.Version), Status: row.Status, Spec: row.Spec,
		SHA256: hex.EncodeToString(row.Sha256), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
	if row.PublishedAt.Valid {
		out.PublishedAt = &row.PublishedAt.Time
	}
	return out
}

var _ designstandards.Store = (*DesignStandardStore)(nil)
