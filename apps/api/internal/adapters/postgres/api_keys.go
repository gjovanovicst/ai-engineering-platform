package postgres

import (
	"context"
	"errors"

	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/apikeys"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// APIKeyStore implements apikeys.Store.
type APIKeyStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAPIKeyStore wires the store.
func NewAPIKeyStore(pool *pgxpool.Pool) *APIKeyStore {
	return &APIKeyStore{pool: pool, q: pgstore.New(pool)}
}

func (s *APIKeyStore) Create(ctx context.Context, orgID, name, role, hash string, scopes []string, expires *time.Time) (apikeys.Key, error) {
	var out apikeys.Key
	var expiresTS pgtype.Timestamptz
	if expires != nil {
		expiresTS = pgtype.Timestamptz{Time: *expires, Valid: true}
	}
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateApiKey(ctx, pgstore.CreateApiKeyParams{
			OrgID: toUUID(orgID), Name: name, KeyHash: []byte(hash), Scopes: scopes, Role: role, ExpiresAt: expiresTS,
		})
		if err != nil {
			return err
		}
		out = apikeys.Key{ID: uuidString(row.ID), OrgID: orgID, Name: row.Name, Role: row.Role, Scopes: scopes, CreatedAt: row.CreatedAt.Time, ExpiresAt: expires}
		return nil
	})
	return out, err
}

func (s *APIKeyStore) ByHash(ctx context.Context, orgID, hash string) (apikeys.Key, error) {
	var out apikeys.Key
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ApiKeyByHash(ctx, pgstore.ApiKeyByHashParams{OrgID: toUUID(orgID), KeyHash: []byte(hash)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = apikeys.Key{ID: uuidString(row.ID), OrgID: orgID, Name: row.Name, Role: row.Role, Scopes: row.Scopes}
		if row.RevokedAt.Valid {
			t := row.RevokedAt.Time
			out.RevokedAt = &t
		}
		if row.ExpiresAt.Valid {
			t := row.ExpiresAt.Time
			out.ExpiresAt = &t
		}
		return nil
	})
	return out, err
}

func (s *APIKeyStore) List(ctx context.Context, orgID string) ([]apikeys.Key, error) {
	var out []apikeys.Key
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListApiKeys(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			key := apikeys.Key{ID: uuidString(row.ID), OrgID: orgID, Name: row.Name, Role: row.Role, Scopes: row.Scopes, CreatedAt: row.CreatedAt.Time}
			if row.RevokedAt.Valid {
				t := row.RevokedAt.Time
				key.RevokedAt = &t
			}
			if row.LastUsedAt.Valid {
				t := row.LastUsedAt.Time
				key.LastUsedAt = &t
			}
			if row.ExpiresAt.Valid {
				t := row.ExpiresAt.Time
				key.ExpiresAt = &t
			}
			out = append(out, key)
		}
		return nil
	})
	return out, err
}

func (s *APIKeyStore) Revoke(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RevokeApiKey(ctx, pgstore.RevokeApiKeyParams{OrgID: toUUID(orgID), ID: toUUID(id)})
	})
}

func (s *APIKeyStore) Touch(ctx context.Context, id string) error {
	return s.q.TouchApiKey(ctx, toUUID(id))
}
