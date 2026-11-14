package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// SearchStore implements search.Store.
type SearchStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewSearchStore wires the store.
func NewSearchStore(pool *pgxpool.Pool) *SearchStore {
	return &SearchStore{pool: pool, q: pgstore.New(pool)}
}

// SearchTasks returns matching tasks, newest first.
func (s *SearchStore) SearchTasks(ctx context.Context, orgID, projectID, query string, limit int) ([]domain.Task, error) {
	var out []domain.Task
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		project := pgtype.UUID{}
		if projectID != "" {
			project = toUUID(projectID)
		}
		rows, err := s.q.WithTx(tx).SearchTasks(ctx, pgstore.SearchTasksParams{
			OrgID: toUUID(orgID), Column2: project, Column3: &query,
			Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]domain.Task, 0, len(rows))
		for _, row := range rows {
			task, terr := taskFromRow(row)
			if terr != nil {
				return terr
			}
			out = append(out, task)
		}
		return nil
	})
	return out, err
}
