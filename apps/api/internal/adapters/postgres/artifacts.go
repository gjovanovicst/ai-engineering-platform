package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runartifacts"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// ArtifactStore implements runartifacts.Store.
type ArtifactStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewArtifactStore wires the store.
func NewArtifactStore(pool *pgxpool.Pool) *ArtifactStore {
	return &ArtifactStore{pool: pool, q: pgstore.New(pool)}
}

// Upsert records artifact metadata.
func (s *ArtifactStore) Upsert(ctx context.Context, orgID, runID, kind, key string, sha []byte, size int64) (runartifacts.Artifact, error) {
	var out runartifacts.Artifact
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertRunArtifact(ctx, pgstore.UpsertRunArtifactParams{
			OrgID: toUUID(orgID), RunID: toUUID(runID), Kind: kind, BucketKey: key, Sha256: sha, Bytes: size,
		})
		if err != nil {
			return err
		}
		out = artifactFromRow(row)
		return nil
	})
	return out, err
}

// ListForRun lists a run's artifacts.
func (s *ArtifactStore) ListForRun(ctx context.Context, orgID, runID string) ([]runartifacts.Artifact, error) {
	var out []runartifacts.Artifact
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRunArtifacts(ctx, pgstore.ListRunArtifactsParams{
			OrgID: toUUID(orgID), RunID: toUUID(runID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, artifactFromRow(row))
		}
		return nil
	})
	return out, err
}

func artifactFromRow(row pgstore.RunArtifact) runartifacts.Artifact {
	return runartifacts.Artifact{
		Kind: row.Kind, BucketKey: row.BucketKey, SHA256: row.Sha256, Bytes: row.Bytes, CreatedAt: row.CreatedAt.Time,
	}
}

var _ runartifacts.Store = (*ArtifactStore)(nil)
