package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/blueprints"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// BlueprintStore persists blueprint versions.
type BlueprintStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewBlueprintStore wires the store.
func NewBlueprintStore(pool *pgxpool.Pool) *BlueprintStore {
	return &BlueprintStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts version 1 of a blueprint.
func (s *BlueprintStore) Create(ctx context.Context, b blueprints.Blueprint) (blueprints.Blueprint, error) {
	var out blueprints.Blueprint
	err := db.WithOrg(ctx, s.pool, b.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateBlueprint(ctx, pgstore.CreateBlueprintParams{
			OrgID: toUUID(b.OrgID), Name: b.Name, Spec: b.Spec,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = blueprintFromRow(row)
		return nil
	})
	return out, err
}

// List returns the org blueprints.
func (s *BlueprintStore) List(ctx context.Context, orgID string, limit int) ([]blueprints.Blueprint, error) {
	var out []blueprints.Blueprint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListBlueprints(ctx, pgstore.ListBlueprintsParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]blueprints.Blueprint, 0, len(rows))
		for _, row := range rows {
			out = append(out, blueprintFromRow(row))
		}
		return nil
	})
	return out, err
}

// Get returns one blueprint.
func (s *BlueprintStore) Get(ctx context.Context, orgID, id string) (blueprints.Blueprint, error) {
	var out blueprints.Blueprint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetBlueprint(ctx, pgstore.GetBlueprintParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = blueprintFromRow(row)
		return nil
	})
	return out, err
}

func blueprintFromRow(row pgstore.Blueprint) blueprints.Blueprint {
	b := blueprints.Blueprint{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), Name: row.Name,
		Version: int(row.Version), Status: row.Status, Spec: row.Spec, CreatedAt: row.CreatedAt.Time,
	}
	if row.PublishedAt.Valid {
		v := row.PublishedAt.Time
		b.PublishedAt = &v
	}
	return b
}

// Publish marks a draft blueprint as published.
func (s *BlueprintStore) Publish(ctx context.Context, orgID, id string) (blueprints.Blueprint, error) {
	var out blueprints.Blueprint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).PublishBlueprint(ctx, pgstore.PublishBlueprintParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTransition
			}
			return err
		}
		out = blueprintFromRow(row)
		return nil
	})
	return out, err
}

// NextVersion returns the next version number for a blueprint name.
func (s *BlueprintStore) NextVersion(ctx context.Context, orgID, name string) (int, error) {
	var out int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).NextBlueprintVersion(ctx, pgstore.NextBlueprintVersionParams{OrgID: toUUID(orgID), Name: name})
		out = int(n)
		return err
	})
	return out, err
}

// CreateVersion inserts a new draft version.
func (s *BlueprintStore) CreateVersion(ctx context.Context, orgID, name string, version int, spec []byte) (blueprints.Blueprint, error) {
	var out blueprints.Blueprint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateBlueprintVersion(ctx, pgstore.CreateBlueprintVersionParams{
			OrgID: toUUID(orgID), Name: name, Version: int32(version), Spec: spec, //nolint:gosec // versions are small
		})
		if err != nil {
			return err
		}
		out = blueprintFromRow(row)
		return nil
	})
	return out, err
}
