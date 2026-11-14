package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/mergequeue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// MergeQueueStore implements mergequeue.Store.
type MergeQueueStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewMergeQueueStore wires the store.
func NewMergeQueueStore(pool *pgxpool.Pool) *MergeQueueStore {
	return &MergeQueueStore{pool: pool, q: pgstore.New(pool)}
}

func (s *MergeQueueStore) Enqueue(ctx context.Context, item mergequeue.Item) (int64, error) {
	var id int64
	err := db.WithOrg(ctx, s.pool, item.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		var run, task, pr pgtype.UUID
		if item.RunID != "" {
			run = toUUID(item.RunID)
		}
		if item.TaskID != "" {
			task = toUUID(item.TaskID)
		}
		if item.PullRequestID != "" {
			pr = toUUID(item.PullRequestID)
		}
		row, err := s.q.WithTx(tx).EnqueueMerge(ctx, pgstore.EnqueueMergeParams{
			OrgID: toUUID(item.OrgID), ProjectID: toUUID(item.ProjectID),
			RunID: run, TaskID: task, PullRequestID: pr, Number: int32(item.Number), //nolint:gosec // PR numbers are small
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // already queued
			}
			return err
		}
		id = row
		return nil
	})
	return id, err
}

func (s *MergeQueueStore) ClaimNext(ctx context.Context, orgID, projectID string) (mergequeue.Item, bool, error) {
	var out mergequeue.Item
	found := false
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ClaimNextMerge(ctx, pgstore.ClaimNextMergeParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		out = mergequeue.Item{
			ID: row.ID, OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
			RunID: uuidString(row.RunID), TaskID: uuidString(row.TaskID), Number: int(row.Number),
			PullRequestID: uuidString(row.PullRequestID),
		}
		found = true
		return nil
	})
	return out, found, err
}

func (s *MergeQueueStore) Complete(ctx context.Context, orgID string, id int64, status, detail string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).CompleteMerge(ctx, pgstore.CompleteMergeParams{
			ID: id, Status: status, Error: detail,
		})
	})
}

func (s *MergeQueueStore) Position(ctx context.Context, projectID string, id int64) (int, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM merge_queue WHERE project_id = $1 AND status = 'queued' AND id <= $2`, projectID, id).Scan(&count)
	return int(count), err
}

// List returns recent queue entries for a project.
func (s *MergeQueueStore) List(ctx context.Context, orgID, projectID string) ([]mergequeue.Item, error) {
	var out []mergequeue.Item
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListMergeQueue(ctx, pgstore.ListMergeQueueParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, mergequeue.Item{
				ID: row.ID, OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
				RunID: uuidString(row.RunID), TaskID: uuidString(row.TaskID), Number: int(row.Number),
				PullRequestID: uuidString(row.PullRequestID),
				Status:        row.Status, Error: row.Error, CreatedAt: row.CreatedAt.Time,
			})
		}
		return nil
	})
	return out, err
}
