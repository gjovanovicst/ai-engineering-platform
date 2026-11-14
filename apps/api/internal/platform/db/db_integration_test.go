//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/migrations"
)

const (
	orgA  = "33333333-3333-3333-3333-333333333333"
	orgB  = "44444444-4444-4444-4444-444444444444"
	userA = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	userB = "dddddddd-dddd-dddd-dddd-dddddddddddd"
)

// TestTenantIsolation proves the second line of defence: even if the
// application code forgets a WHERE org_id clause, PostgreSQL RLS returns no
// rows for the wrong tenant.
func TestTenantIsolation(t *testing.T) {
	appDSN := os.Getenv("AIP_TEST_DB_DSN")
	migDSN := os.Getenv("AIP_TEST_DB_MIGRATOR_DSN")
	if appDSN == "" || migDSN == "" {
		t.Skip("AIP_TEST_DB_DSN / AIP_TEST_DB_MIGRATOR_DSN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Migrate(ctx, migDSN, migrations.FS, "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	migrator, err := db.Connect(ctx, migDSN)
	if err != nil {
		t.Fatalf("connect as migrator: %v", err)
	}
	app, err := db.Connect(ctx, appDSN)
	if err != nil {
		migrator.Close()
		t.Fatalf("connect as app: %v", err)
	}
	t.Cleanup(func() {
		app.Close()
		migrator.Close()
		if err := db.Migrate(context.Background(), migDSN, migrations.FS, "down"); err != nil {
			t.Logf("migrate down: %v", err)
		}
	})

	seeds := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO orgs (id, slug, name) VALUES ($1, 'acme', 'Acme'), ($2, 'globex', 'Globex')`, []any{orgA, orgB}},
		{`INSERT INTO users (id, email, name) VALUES ($1, 'a@acme.io', 'A'), ($2, 'b@globex.io', 'B')`, []any{userA, userB}},
		{`INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $3, 'owner'), ($2, $4, 'owner')`, []any{orgA, orgB, userA, userB}},
		{`INSERT INTO audit_log (org_id, actor_kind, action, target) VALUES ($1, 'system', 'org.bootstrap', 'org:acme')`, []any{orgA}},
	}
	for _, s := range seeds {
		if _, err := migrator.Exec(ctx, s.query, s.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	t.Run("with org context sees only its rows", func(t *testing.T) {
		var n int
		err := db.WithOrg(ctx, app, orgA, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT count(*) FROM memberships").Scan(&n)
		})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		if n != 1 {
			t.Fatalf("memberships visible = %d, want 1", n)
		}
	})

	t.Run("without org context sees nothing", func(t *testing.T) {
		var n int
		if err := app.QueryRow(ctx, "SELECT count(*) FROM memberships").Scan(&n); err != nil {
			t.Fatalf("query: %v", err)
		}
		if n != 0 {
			t.Fatalf("memberships visible without org = %d, want 0", n)
		}
	})

	t.Run("audit log is append-only", func(t *testing.T) {
		if _, err := app.Exec(ctx, "UPDATE audit_log SET action = 'tampered'"); err == nil {
			t.Fatal("UPDATE audit_log succeeded, want permission denied")
		}
	})
}
