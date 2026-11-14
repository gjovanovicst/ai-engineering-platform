// Package db owns the PostgreSQL connection pool and the transaction helpers
// that enforce tenancy (RLS) and isolation between use cases.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool and verifies connectivity. It fails fast: a process
// that cannot reach its database must not pretend to be healthy.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// TxFunc is a unit of work that runs inside a transaction.
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// WithTx runs fn in a transaction and commits only when fn returns nil.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		// Rollback after a successful commit returns ErrTxClosed; that is expected.
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// WithOrg runs fn in a transaction with app.org_id set for the duration of that
// transaction. Every query touching a tenant table must go through this helper;
// PostgreSQL RLS then guarantees the second line of defence.
func WithOrg(ctx context.Context, pool *pgxpool.Pool, orgID string, fn TxFunc) error {
	if orgID == "" {
		return errors.New("db: empty org id")
	}
	return WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		// set_config(..., true) is transaction-local, so pooled connections do
		// not leak the tenant to the next request.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", orgID); err != nil {
			return fmt.Errorf("set app.org_id: %w", err)
		}
		return fn(ctx, tx)
	})
}
