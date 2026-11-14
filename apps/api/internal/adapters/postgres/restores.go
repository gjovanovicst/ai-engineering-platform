package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/restores"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// RestoreStore implements restores.Store.
type RestoreStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewRestoreStore wires the store.
func NewRestoreStore(pool *pgxpool.Pool) *RestoreStore {
	return &RestoreStore{pool: pool, q: pgstore.New(pool)}
}

func (s *RestoreStore) Create(ctx context.Context, orgID, actorID string, target time.Time) (restores.Request, error) {
	var out restores.Request
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateRestoreRequest(ctx, pgstore.CreateRestoreRequestParams{
			OrgID: toUUID(orgID), TargetTime: pgTime(target), RequestedBy: toOptionalUUID(&actorID),
		})
		if err != nil {
			return err
		}
		out = restores.Request{ID: uuidString(row.ID), TargetTime: row.TargetTime.Time, Status: row.Status, Detail: row.Detail, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}

func (s *RestoreStore) List(ctx context.Context, orgID string) ([]restores.Request, error) {
	var out []restores.Request
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRestoreRequests(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, restores.Request{ID: uuidString(row.ID), TargetTime: row.TargetTime.Time, Status: row.Status, Detail: row.Detail, CreatedAt: row.CreatedAt.Time})
		}
		return nil
	})
	return out, err
}

func (s *RestoreStore) DR(ctx context.Context, orgID string) (restores.DR, error) {
	var out restores.DR
	err := s.pool.QueryRow(ctx, `SELECT failed_count, archived_count,
        COALESCE(EXTRACT(EPOCH FROM now()-last_archived_time),0)::float8
        FROM pg_stat_archiver`).Scan(&out.WALFailed, &out.WALArchived, &out.WALAge)
	if err != nil {
		return out, err
	}
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(EXTRACT(EPOCH FROM now()-last_success_at),0)::float8, detail
        FROM backup_heartbeat WHERE kind='base'`).Scan(&out.BaseAge, &out.BaseName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(EXTRACT(EPOCH FROM now()-last_success_at),0)::float8
        FROM backup_heartbeat WHERE kind='offsite'`).Scan(&out.OffsiteAge)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	err = db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT
            count(*) FILTER (WHERE status='succeeded')::bigint,
            count(*) FILTER (WHERE status='failed')::bigint,
            COALESCE(EXTRACT(EPOCH FROM now()-max(updated_at) FILTER (WHERE status='succeeded')),0)::float8,
            COALESCE(avg(EXTRACT(EPOCH FROM updated_at-created_at)) FILTER (WHERE status='succeeded'),0)::float8
            FROM restore_requests WHERE org_id=$1`, toUUID(orgID)).Scan(&out.RestoreSucceeded, &out.RestoreFailed, &out.LastRestoreAge, &out.AvgRestoreSeconds)
	})
	return out, err
}

func (s *RestoreStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}

var _ restores.Store = (*RestoreStore)(nil)
