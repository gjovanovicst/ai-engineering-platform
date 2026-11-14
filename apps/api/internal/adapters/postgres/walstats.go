package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WALStats reads PostgreSQL's WAL archiver counters.
type WALStats struct{ pool *pgxpool.Pool }

// NewWALStats wires the reader.
func NewWALStats(pool *pgxpool.Pool) *WALStats { return &WALStats{pool: pool} }

// Stats implements httpapi.WALArchiveStats.
func (w *WALStats) Stats(ctx context.Context) (failed, archived int64, lastAge float64, err error) {
	err = w.pool.QueryRow(ctx, `SELECT failed_count, archived_count,
        COALESCE(EXTRACT(EPOCH FROM now() - last_archived_time), 0)::float8
        FROM pg_stat_archiver`).Scan(&failed, &archived, &lastAge)
	return
}
