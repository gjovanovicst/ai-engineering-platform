package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
)

// Migrate applies goose commands ("up", "down", "status", "version") from an
// embedded filesystem. It must run with the migrator role, never with the
// application role.
func Migrate(ctx context.Context, dsn string, fsys fs.FS, command string) error {
	if len(command) == 0 {
		command = "up"
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open migrator connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("ping as migrator: %w", err)
	}

	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}

	if err := goose.RunContext(ctx, command, conn, "."); err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}
	return nil
}
