package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checksync"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// PRSyncStore lists open pull requests across all orgs for checksync.
type PRSyncStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewPRSyncStore wires the store.
func NewPRSyncStore(pool *pgxpool.Pool) *PRSyncStore {
	return &PRSyncStore{pool: pool, q: pgstore.New(pool)}
}

// ListOpen returns open PRs; the org loop keeps RLS enforced per tenant.
func (s *PRSyncStore) ListOpen(ctx context.Context) ([]checksync.OpenPR, error) {
	rows, err := s.pool.Query(ctx, "SELECT id::text FROM orgs ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	var orgs []string
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			rows.Close()
			return nil, err
		}
		orgs = append(orgs, orgID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []checksync.OpenPR
	for _, orgID := range orgs {
		var items []checksync.OpenPR
		err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
			list, err := s.q.WithTx(tx).ListOpenPullRequestsForSync(ctx)
			if err != nil {
				return err
			}
			for _, item := range list {
				items = append(items, checksync.OpenPR{
					ID: uuidString(item.ID), OrgID: uuidString(item.OrgID), RepoID: uuidString(item.RepoID),
					RepoName: item.RepoName, Provider: item.Provider, Number: int(item.Number),
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}
