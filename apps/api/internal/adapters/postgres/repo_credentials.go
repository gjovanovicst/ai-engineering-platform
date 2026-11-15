package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/repocreds"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// RepoCredentialStore persists encrypted per-repository git credentials.
type RepoCredentialStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewRepoCredentialStore wires the store.
func NewRepoCredentialStore(pool *pgxpool.Pool) *RepoCredentialStore {
	return &RepoCredentialStore{pool: pool, q: pgstore.New(pool)}
}

// Upsert seals a credential and appends an audit entry in the same transaction.
func (s *RepoCredentialStore) Upsert(ctx context.Context, orgID, repoID, actorID, action string, rec repocreds.Record) (repocreds.Record, error) {
	var out repocreds.Record
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertRepoCredential(ctx, pgstore.UpsertRepoCredentialParams{
			OrgID: toUUID(orgID), RepoID: toUUID(repoID),
			Ciphertext: rec.Ciphertext, Hint: rec.Hint, ActorID: toUUID(actorID),
		})
		if err != nil {
			return err
		}
		out = credentialFromRow(row)
		return recordCredentialAudit(ctx, s.q.WithTx(tx), orgID, repoID, actorID, action, rec.Hint)
	})
	return out, err
}

// Get returns the stored credential or domain.ErrNotFound.
func (s *RepoCredentialStore) Get(ctx context.Context, orgID, repoID string) (repocreds.Record, error) {
	var out repocreds.Record
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRepoCredential(ctx, pgstore.GetRepoCredentialParams{
			OrgID: toUUID(orgID), RepoID: toUUID(repoID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = credentialFromRow(row)
		return nil
	})
	return out, err
}

// Delete removes a credential and audits the removal; a missing row is a no-op.
func (s *RepoCredentialStore) Delete(ctx context.Context, orgID, repoID, actorID, action string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		deleted, err := s.q.WithTx(tx).DeleteRepoCredential(ctx, pgstore.DeleteRepoCredentialParams{
			OrgID: toUUID(orgID), RepoID: toUUID(repoID),
		})
		if err != nil {
			return err
		}
		if deleted == 0 {
			return nil
		}
		return recordCredentialAudit(ctx, s.q.WithTx(tx), orgID, repoID, actorID, action, "")
	})
}

func recordCredentialAudit(ctx context.Context, q *pgstore.Queries, orgID, repoID, actorID, action, hint string) error {
	meta, err := json.Marshal(map[string]string{"repo_id": repoID, "hint": hint})
	if err != nil {
		return err
	}
	return q.RecordRepoCredentialAudit(ctx, pgstore.RecordRepoCredentialAuditParams{
		OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action,
		Target: "repo:" + repoID, Metadata: meta,
	})
}

func credentialFromRow(row pgstore.RepoCredential) repocreds.Record {
	return repocreds.Record{
		RepoID: uuidString(row.RepoID), Ciphertext: row.Ciphertext, Hint: row.Hint,
		CreatedBy: uuidString(row.CreatedBy), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}
