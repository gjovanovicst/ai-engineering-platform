//go:build integration

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/migrations"
)

func migratedPool(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool, context.Context) {
	t.Helper()
	appDSN := os.Getenv("AIP_TEST_DB_DSN")
	migDSN := os.Getenv("AIP_TEST_DB_MIGRATOR_DSN")
	if appDSN == "" || migDSN == "" {
		t.Skip("AIP_TEST_DB_DSN / AIP_TEST_DB_MIGRATOR_DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	if err := db.Migrate(ctx, migDSN, migrations.FS, "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	app, err := db.Connect(ctx, appDSN)
	if err != nil {
		t.Fatalf("connect app: %v", err)
	}
	t.Cleanup(app.Close)
	migrator, err := db.Connect(ctx, migDSN)
	if err != nil {
		t.Fatalf("connect migrator: %v", err)
	}
	t.Cleanup(migrator.Close)
	t.Cleanup(func() {
		if err := db.Migrate(context.Background(), migDSN, migrations.FS, "down"); err != nil {
			t.Logf("migrate down: %v", err)
		}
	})
	return app, migrator, ctx
}

// deleteDemoData removes the seeded rows. The integration database is shared
// by the packages (cmd/aipctl runs before internal/...), and the session test
// inserts an org with the same "acme" slug, so a leftover demo org would make
// it fail.
func deleteDemoData(t *testing.T, migrator *pgxpool.Pool, ctx context.Context, s seedSummary) {
	t.Helper()
	stmts := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM tasks WHERE project_id IN (SELECT id FROM projects WHERE org_id=$1)`, []any{s.OrgID}},
		{`DELETE FROM projects WHERE org_id=$1`, []any{s.OrgID}},
		{`DELETE FROM memberships WHERE org_id=$1`, []any{s.OrgID}},
		{`DELETE FROM users WHERE id=$1`, []any{s.UserID}},
		{`DELETE FROM orgs WHERE id=$1`, []any{s.OrgID}},
	}
	for _, stmt := range stmts {
		if _, err := migrator.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Errorf("cleanup demo data: %v", err)
		}
	}
}

func TestSeedCreatesTheDemoDataset(t *testing.T) {
	pool, migrator, ctx := migratedPool(t)
	summary, err := seedDemoData(ctx, pool, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteDemoData(t, migrator, ctx, summary) })
	if summary.OrgID == "" || summary.UserID == "" || summary.ProjectID == "" {
		t.Fatalf("incomplete summary: %+v", summary)
	}
	if summary.Tasks != 3 {
		t.Fatalf("tasks = %d, want 3", summary.Tasks)
	}

	accounts := postgres.NewAccountStore(pool)
	user, _, err := accounts.UserByEmail(ctx, seedEmail)
	if err != nil {
		t.Fatalf("demo user: %v", err)
	}
	if user.ID != summary.UserID || !user.EmailVerified {
		t.Fatalf("user = %+v, want the verified demo user", user)
	}
	projects := postgres.NewProjectStore(pool)
	list, err := projects.List(ctx, summary.OrgID, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range list {
		if p.Slug == seedProjectSlug && p.ID == summary.ProjectID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the demo project is missing from %+v", list)
	}
	tasks := postgres.NewTaskStore(pool)
	rows, err := tasks.List(ctx, summary.OrgID, summary.ProjectID, "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("listed %d tasks, want 3", len(rows))
	}
	for _, task := range rows {
		if len(task.Acceptance) == 0 || !task.Acceptance[0].Complete() {
			t.Fatalf("task %q has no complete acceptance criterion", task.Ref)
		}
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	pool, migrator, ctx := migratedPool(t)
	first, err := seedDemoData(ctx, pool, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deleteDemoData(t, migrator, ctx, first) })
	second, err := seedDemoData(ctx, pool, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if first.OrgID != second.OrgID || first.UserID != second.UserID || first.ProjectID != second.ProjectID {
		t.Fatalf("the second run changed the ids: %+v vs %+v", first, second)
	}
	if second.Tasks != 3 {
		t.Fatalf("tasks after the second run = %d, want 3 (no duplicates)", second.Tasks)
	}
}
