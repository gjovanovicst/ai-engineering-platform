//go:build integration

package postgres_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
	"github.com/gjovanovicst/ai-engineering-platform/migrations"
)

const (
	testOrg  = "11111111-1111-1111-1111-111111111111"
	testUser = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

func TestSessionStoreLifecycle(t *testing.T) {
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

	if _, err := migrator.Exec(ctx,
		`INSERT INTO orgs (id, slug, name) VALUES ($1, 'acme', 'Acme')`, testOrg); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if _, err := migrator.Exec(ctx,
		`INSERT INTO users (id, email, name) VALUES ($1, 'a@acme.io', 'A')`, testUser); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	issuer, err := auth.NewIssuer(testKeyPEM(t), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.NewService(postgres.NewSessionStore(app), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	first, err := svc.Issue(ctx, auth.IssueRequest{UserID: testUser, OrgID: testOrg, Role: "owner"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if activeSessions(t, ctx, app) != 1 {
		t.Fatalf("active sessions = %d, want 1", activeSessions(t, ctx, app))
	}

	second, err := svc.Refresh(ctx, first.RefreshToken, "")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if activeSessions(t, ctx, app) != 1 {
		t.Fatalf("active sessions after rotation = %d, want 1", activeSessions(t, ctx, app))
	}

	claims, err := issuer.Parse(second.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.OrgID != testOrg || claims.Role != "owner" {
		t.Fatalf("claims after refresh = %+v", claims)
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken, ""); !errors.Is(err, auth.ErrRefreshReuse) {
		t.Fatalf("reuse err = %v, want ErrRefreshReuse", err)
	}
	if activeSessions(t, ctx, app) != 0 {
		t.Fatalf("active sessions after reuse = %d, want 0", activeSessions(t, ctx, app))
	}
}

func activeSessions(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return n
}

func testKeyPEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
