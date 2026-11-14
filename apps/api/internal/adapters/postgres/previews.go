package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/previews"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// PreviewStore implements previews.Store.
type PreviewStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewPreviewStore wires the store.
func NewPreviewStore(pool *pgxpool.Pool) *PreviewStore {
	return &PreviewStore{pool: pool, q: pgstore.New(pool)}
}

func previewFromRow(row pgstore.PreviewEnvironment) previews.Environment {
	return previews.Environment{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
		Name: row.Name, Image: row.Image, Port: int(row.Port), Status: row.Status,
		URL: row.Url, ContainerID: row.ContainerID, Error: row.Error,
		ExpiresAt: row.ExpiresAt.Time, CreatedAt: row.CreatedAt.Time,
	}
}

func (s *PreviewStore) Create(ctx context.Context, env previews.Environment) (previews.Environment, error) {
	var out previews.Environment
	err := db.WithOrg(ctx, s.pool, env.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreatePreview(ctx, pgstore.CreatePreviewParams{
			OrgID: toUUID(env.OrgID), ProjectID: toUUID(env.ProjectID), Name: env.Name,
			Image: env.Image, Port: int32(env.Port), //nolint:gosec // validated 1..65535
			ExpiresAt: pgtype.Timestamptz{Time: env.ExpiresAt, Valid: true},
		})
		if err != nil {
			return err
		}
		out = previewFromRow(row)
		return nil
	})
	return out, err
}

func (s *PreviewStore) List(ctx context.Context, orgID, projectID string) ([]previews.Environment, error) {
	var out []previews.Environment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListPreviews(ctx, pgstore.ListPreviewsParams{OrgID: toUUID(orgID), ProjectID: toUUID(projectID)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, previewFromRow(row))
		}
		return nil
	})
	return out, err
}

func (s *PreviewStore) Get(ctx context.Context, orgID, id string) (previews.Environment, error) {
	var out previews.Environment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetPreview(ctx, pgstore.GetPreviewParams{OrgID: toUUID(orgID), ID: toUUID(id)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = previewFromRow(row)
		return nil
	})
	return out, err
}

func (s *PreviewStore) SetStatus(ctx context.Context, orgID, id, status, rawURL, containerID, detail string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetPreviewStatus(ctx, pgstore.SetPreviewStatusParams{
			OrgID: toUUID(orgID), ID: toUUID(id), Status: status, Url: rawURL, ContainerID: containerID, Error: detail,
		})
	})
}

var _ = time.Second
