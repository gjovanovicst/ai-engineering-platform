package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/githubsetup"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// GithubAppStore implements githubsetup.Store.
type GithubAppStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewGithubAppStore wires the store.
func NewGithubAppStore(pool *pgxpool.Pool) *GithubAppStore {
	return &GithubAppStore{pool: pool, q: pgstore.New(pool)}
}

func (s *GithubAppStore) Upsert(ctx context.Context, orgID string, in githubsetup.Record) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var installation *int64
		if in.InstallationID > 0 {
			installation = &in.InstallationID
		}
		_, err := s.q.WithTx(tx).UpsertGithubApp(ctx, pgstore.UpsertGithubAppParams{
			OrgID: toUUID(orgID), AppID: in.AppID, Slug: in.Slug,
			PemEncrypted: in.PEM, SecretEncrypted: in.WebhookSecret,
			InstallUrl: in.InstallURL, InstallationID: installation,
		})
		return err
	})
}

func (s *GithubAppStore) Get(ctx context.Context, orgID string) (githubsetup.Record, error) {
	var out githubsetup.Record
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetGithubApp(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = githubsetup.Record{
			AppID: row.AppID, Slug: row.Slug, PEM: row.PemEncrypted,
			WebhookSecret: row.SecretEncrypted, InstallURL: row.InstallUrl,
		}
		if row.InstallationID != nil {
			out.InstallationID = *row.InstallationID
		}
		return nil
	})
	return out, err
}

func (s *GithubAppStore) SetInstallation(ctx context.Context, orgID string, installationID int64) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.q.WithTx(tx).SetGithubInstallation(ctx, pgstore.SetGithubInstallationParams{
			OrgID: toUUID(orgID), InstallationID: &installationID,
		})
		return err
	})
}

func (s *GithubAppStore) CreateState(ctx context.Context, state, orgID string) error {
	return s.q.CreateGithubAppState(ctx, pgstore.CreateGithubAppStateParams{
		State: state, OrgID: toUUID(orgID),
	})
}

func (s *GithubAppStore) ConsumeState(ctx context.Context, state string) (string, error) {
	row, err := s.q.GetGithubAppState(ctx, state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	_ = s.q.DeleteGithubAppState(ctx, state)
	return uuidString(row.OrgID), nil
}
