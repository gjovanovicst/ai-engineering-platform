package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type writablePool struct{ pool *pgxpool.Pool }

// Writable fails when the connected database is in recovery/read-only.
func (w writablePool) Writable(ctx context.Context) error {
	var recovery bool
	if err := w.pool.QueryRow(ctx, "select pg_is_in_recovery()").Scan(&recovery); err != nil {
		return err
	}
	if recovery {
		return errors.New("database is in recovery")
	}
	return nil
}
