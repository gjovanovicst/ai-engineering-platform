package postgres

import (
	"context"
	"errors"

	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/scim"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// SCIMStore implements scim.Store.
type SCIMStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewSCIMStore wires the store.
func NewSCIMStore(pool *pgxpool.Pool) *SCIMStore {
	return &SCIMStore{pool: pool, q: pgstore.New(pool)}
}

func (s *SCIMStore) CreateToken(ctx context.Context, orgID, name, hash string) (scim.Token, error) {
	var out scim.Token
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateScimToken(ctx, pgstore.CreateScimTokenParams{
			OrgID: toUUID(orgID), Name: name, TokenHash: []byte(hash),
		})
		if err != nil {
			return err
		}
		out = scim.Token{ID: uuidString(row.ID), Name: row.Name, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}

func (s *SCIMStore) ListTokens(ctx context.Context, orgID string) ([]scim.Token, error) {
	var out []scim.Token
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListScimTokens(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			token := scim.Token{ID: uuidString(row.ID), Name: row.Name, CreatedAt: row.CreatedAt.Time}
			if row.RevokedAt.Valid {
				value := row.RevokedAt.Time
				token.RevokedAt = &value
			}
			out = append(out, token)
		}
		return nil
	})
	return out, err
}

func (s *SCIMStore) RevokeToken(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := s.q.WithTx(tx).RevokeScimToken(ctx, pgstore.RevokeScimTokenParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			return err
		}
		if tag == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (s *SCIMStore) TokenOrg(ctx context.Context, orgID, hash string) (string, error) {
	var org string
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ScimTokenByHash(ctx, []byte(hash))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		org = uuidString(row)
		return nil
	})
	return org, err
}

func (s *SCIMStore) Users(ctx context.Context, orgID string) ([]scim.User, error) {
	var out []scim.User
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListScimUsers(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, scim.User{ID: uuidString(row.ID), Email: row.Email, Name: row.Name, CreatedAt: row.CreatedAt.Time})
		}
		return nil
	})
	return out, err
}

// OwnerID returns the org owner (the actor for invite-backed provisioning).
func (s *SCIMStore) OwnerID(ctx context.Context, orgID string) (string, error) {
	var owner string
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SCIMOwner(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		owner = uuidString(row)
		return nil
	})
	return owner, err
}

// MembershipRole returns the user's org role.
func (s *SCIMStore) MembershipRole(ctx context.Context, orgID, userID string) (string, error) {
	var role string
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		value, err := s.q.WithTx(tx).SCIMMembershipRole(ctx, pgstore.SCIMMembershipRoleParams{OrgID: toUUID(orgID), UserID: toUUID(userID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		role = value
		return nil
	})
	return role, err
}

// RemoveMembership revokes org access.
func (s *SCIMStore) RemoveMembership(ctx context.Context, orgID, userID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SCIMRemoveMembership(ctx, pgstore.SCIMRemoveMembershipParams{OrgID: toUUID(orgID), UserID: toUUID(userID)})
	})
}

// UpdateMembershipRole changes a membership role.
func (s *SCIMStore) UpdateMembershipRole(ctx context.Context, orgID, userID, role string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).SCIMUpdateMembershipRole(ctx, pgstore.SCIMUpdateMembershipRoleParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Role: role,
		})
		if err != nil {
			return err
		}
		if rows == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// UpdateUserName changes a member's display name.
func (s *SCIMStore) UpdateUserName(ctx context.Context, orgID, userID, name string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).SCIMUpdateUserName(ctx, pgstore.SCIMUpdateUserNameParams{
			OrgID: toUUID(orgID), ID: toUUID(userID), Name: name,
		})
		if err != nil {
			return err
		}
		if rows == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// UpdateUserEmail changes a member's login email.
func (s *SCIMStore) UpdateUserEmail(ctx context.Context, orgID, userID, email string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).SCIMUpdateUserEmail(ctx, pgstore.SCIMUpdateUserEmailParams{
			OrgID: toUUID(orgID), ID: toUUID(userID), Email: email,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("%w: email is already in use", domain.ErrValidation)
			}
			return err
		}
		if rows == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}
