//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/repocreds"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/migrations"
)

const (
	credOrg     = "33333333-3333-3333-3333-333333333333"
	credUser    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	credProject = "44444444-4444-4444-4444-444444444444"
	credRepo    = "55555555-5555-5555-5555-555555555555"
)

func TestRepoCredentialStoreLifecycle(t *testing.T) {
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
		t.Fatalf("connect migrator: %v", err)
	}
	app, err := db.Connect(ctx, appDSN)
	if err != nil {
		migrator.Close()
		t.Fatalf("connect app: %v", err)
	}
	t.Cleanup(func() {
		app.Close()
		migrator.Close()
		if err := db.Migrate(context.Background(), migDSN, migrations.FS, "down"); err != nil {
			t.Logf("migrate down: %v", err)
		}
	})

	seed := func(query string, args ...any) {
		t.Helper()
		if _, err := migrator.Exec(ctx, query, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	seed(`INSERT INTO orgs (id, slug, name) VALUES ($1, 'cred-acme', 'Cred Acme')`, credOrg)
	seed(`INSERT INTO users (id, email, name) VALUES ($1, 'cred@acme.io', 'Cred')`, credUser)
	seed(`INSERT INTO projects (id, org_id, name, slug) VALUES ($1, $2, 'Cred', 'cred')`, credProject, credOrg)
	seed(`INSERT INTO repos (id, org_id, project_id, provider, name, webhook_secret)
	      VALUES ($1, $2, $3, 'github', 'acme/cred', 's3cret')`, credRepo, credOrg, credProject)

	store := postgres.NewRepoCredentialStore(app)

	stored, err := store.Upsert(ctx, credOrg, credRepo, credUser, "repo.credential.set", repocreds.Record{
		RepoID: credRepo, Ciphertext: []byte("sealed-token"), Hint: "…1234",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if string(stored.Ciphertext) != "sealed-token" || stored.Hint != "…1234" {
		t.Fatalf("stored = %+v", stored)
	}
	if stored.UpdatedAt.IsZero() {
		t.Fatal("updated_at was not set")
	}

	got, err := store.Get(ctx, credOrg, credRepo)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Ciphertext) != "sealed-token" {
		t.Fatalf("ciphertext = %q", got.Ciphertext)
	}

	// The upsert and the audit row are one transaction.
	var audits int
	if err := migrator.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'repo.credential.set'`, credOrg).Scan(&audits); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if audits != 1 {
		t.Fatalf("audit rows = %d, want 1", audits)
	}

	// Updating keeps one row and bumps updated_at.
	if _, err := store.Upsert(ctx, credOrg, credRepo, credUser, "repo.credential.set", repocreds.Record{
		RepoID: credRepo, Ciphertext: []byte("sealed-token-2"), Hint: "…5678",
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err = store.Get(ctx, credOrg, credRepo)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if string(got.Ciphertext) != "sealed-token-2" || got.Hint != "…5678" {
		t.Fatalf("after update = %+v", got)
	}

	// An actor without a users row (API key subject) must not fail the write:
	// created_by falls back to NULL.
	stored, err = store.Upsert(ctx, credOrg, credRepo, "99999999-9999-4999-8999-999999999999", "repo.credential.set", repocreds.Record{
		RepoID: credRepo, Ciphertext: []byte("sealed-token-3"), Hint: "…9999",
	})
	if err != nil {
		t.Fatalf("upsert with unknown actor: %v", err)
	}
	if stored.CreatedBy != "" {
		t.Fatalf("created_by = %q, want empty for an unknown actor", stored.CreatedBy)
	}

	// Another org cannot read the credential (RLS + org predicate).
	if _, err := store.Get(ctx, "66666666-6666-6666-6666-666666666666", credRepo); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-org get = %v, want ErrNotFound", err)
	}

	if err := store.Delete(ctx, credOrg, credRepo, credUser, "repo.credential.deleted"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(ctx, credOrg, credRepo); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
	// Deleting an absent credential is a no-op and is not audited twice.
	if err := store.Delete(ctx, credOrg, credRepo, credUser, "repo.credential.deleted"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if err := migrator.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE org_id = $1 AND action = 'repo.credential.deleted'`, credOrg).Scan(&audits); err != nil {
		t.Fatalf("count delete audit: %v", err)
	}
	if audits != 1 {
		t.Fatalf("delete audit rows = %d, want 1", audits)
	}
}
