package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/sso"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// SSOStore implements sso.Store.
type SSOStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewSSOStore wires the store.
func NewSSOStore(pool *pgxpool.Pool) *SSOStore { return &SSOStore{pool: pool, q: pgstore.New(pool)} }

func (s *SSOStore) Get(ctx context.Context, orgID string) (sso.Config, error) {
	var out sso.Config
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetOrgSSO(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = sso.Config{EntityID: row.EntityID, MetadataURL: row.MetadataUrl, SSOURL: row.SsoUrl, CertificatePEM: row.CertificatePem, Enabled: row.Enabled, UpdatedAt: row.UpdatedAt.Time}
		return nil
	})
	return out, err
}

func (s *SSOStore) Upsert(ctx context.Context, orgID string, cfg sso.Config) (sso.Config, error) {
	var out sso.Config
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertOrgSSO(ctx, pgstore.UpsertOrgSSOParams{
			OrgID: toUUID(orgID), EntityID: cfg.EntityID, MetadataUrl: cfg.MetadataURL,
			CertificatePem: cfg.CertificatePEM, SsoUrl: cfg.SSOURL, Enabled: cfg.Enabled,
		})
		if err != nil {
			return err
		}
		out = sso.Config{EntityID: row.EntityID, MetadataURL: row.MetadataUrl, SSOURL: row.SsoUrl, CertificatePEM: row.CertificatePem, Enabled: row.Enabled, UpdatedAt: row.UpdatedAt.Time}
		return nil
	})
	return out, err
}

func (s *SSOStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}
