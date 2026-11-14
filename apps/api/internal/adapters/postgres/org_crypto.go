package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/orgcrypto"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// OrgCryptoStore implements orgcrypto.Store.
type OrgCryptoStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewOrgCryptoStore wires the store.
func NewOrgCryptoStore(pool *pgxpool.Pool) *OrgCryptoStore {
	return &OrgCryptoStore{pool: pool, q: pgstore.New(pool)}
}

func (s *OrgCryptoStore) Get(ctx context.Context, orgID string) (orgcrypto.Config, error) {
	var out orgcrypto.Config
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetOrgCrypto(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = orgcrypto.Config{KMSURL: row.KmsUrl, KeyID: row.KeyID, TokenEnc: row.KmsTokenEnc, Status: row.Status, UpdatedAt: row.UpdatedAt.Time}
		return nil
	})
	return out, err
}

func (s *OrgCryptoStore) Upsert(ctx context.Context, orgID, kmsURL, keyID string, tokenEnc []byte) (orgcrypto.Config, error) {
	var out orgcrypto.Config
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertOrgCrypto(ctx, pgstore.UpsertOrgCryptoParams{
			OrgID: toUUID(orgID), KmsUrl: kmsURL, KeyID: keyID, KmsTokenEnc: tokenEnc,
		})
		if err != nil {
			return err
		}
		out = orgcrypto.Config{KMSURL: row.KmsUrl, KeyID: row.KeyID, TokenEnc: row.KmsTokenEnc, Status: row.Status, UpdatedAt: row.UpdatedAt.Time}
		return nil
	})
	return out, err
}

func (s *OrgCryptoStore) Disable(ctx context.Context, orgID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DisableOrgCrypto(ctx, toUUID(orgID))
	})
}

func (s *OrgCryptoStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}
