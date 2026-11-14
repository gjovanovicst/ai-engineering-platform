package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BackupStats reads the latest base-backup heartbeat.
type BackupStats struct{ pool *pgxpool.Pool }

// NewBackupStats wires the reader.
func NewBackupStats(pool *pgxpool.Pool) *BackupStats { return &BackupStats{pool: pool} }

// LastRestoreAge implements httpapi.BackupStats.
func (b *BackupStats) LastRestoreAge(ctx context.Context) (float64, error) {
	var age float64
	err := b.pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - last_success_at)::float8
        FROM backup_heartbeat WHERE kind = 'restore'`).Scan(&age)
	return age, err
}

// LastOffsiteAge implements httpapi.BackupStats.
func (b *BackupStats) LastOffsiteAge(ctx context.Context) (float64, error) {
	var age float64
	err := b.pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - last_success_at)::float8
        FROM backup_heartbeat WHERE kind = 'offsite'`).Scan(&age)
	return age, err
}

// LastBaseBackupAge implements httpapi.BackupStats.
func (b *BackupStats) LastBaseBackupAge(ctx context.Context) (float64, error) {
	var age float64
	err := b.pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM now() - last_success_at)::float8
        FROM backup_heartbeat WHERE kind = 'base'`).Scan(&age)
	return age, err
}
