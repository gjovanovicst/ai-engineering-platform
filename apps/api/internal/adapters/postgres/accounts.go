package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AccountStore implements auth.AccountStore on PostgreSQL.
type AccountStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAccountStore wires the store to a pool.
func NewAccountStore(pool *pgxpool.Pool) *AccountStore {
	return &AccountStore{pool: pool, q: pgstore.New(pool)}
}

var _ auth.AccountStore = (*AccountStore)(nil)

// OrgSlugExists reports whether the slug is already taken.
func (s *AccountStore) OrgSlugExists(ctx context.Context, slug string) (bool, error) {
	return s.q.OrgSlugExists(ctx, slug)
}

// CreateAccount creates the user, org and owner membership in one transaction.
func (s *AccountStore) CreateAccount(ctx context.Context, p auth.CreateAccountParams) (domain.User, domain.Org, error) {
	var user domain.User
	var org domain.Org

	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		hash := p.PasswordHash
		row, err := q.CreateUser(ctx, pgstore.CreateUserParams{
			Email:        p.Email,
			Name:         p.Name,
			PasswordHash: &hash,
		})
		if err != nil {
			return err
		}
		user = userFromRow(row)
		if p.Verified {
			if err := q.MarkEmailVerified(ctx, pgstore.MarkEmailVerifiedParams{
				ID: row.ID, EmailVerifiedAt: toTS(time.Now().UTC()),
			}); err != nil {
				return err
			}
			user.EmailVerified = true
		}

		orgRow, err := q.CreateOrg(ctx, pgstore.CreateOrgParams{Slug: p.OrgSlug, Name: p.OrgName})
		if err != nil {
			return err
		}
		org = domain.Org{
			ID: uuidString(orgRow.ID), Slug: orgRow.Slug, Name: orgRow.Name,
			MaxConcurrentRuns: int(orgRow.MaxConcurrentRuns),
			CreatedAt:         orgRow.CreatedAt.Time, UpdatedAt: orgRow.UpdatedAt.Time,
		}

		// The memberships table is protected by RLS; the tenant for this
		// transaction is the org we just created.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", uuidString(orgRow.ID)); err != nil {
			return err
		}
		if err := q.AddMembership(ctx, pgstore.AddMembershipParams{
			OrgID: orgRow.ID, UserID: row.ID, Role: "owner",
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.User{}, domain.Org{}, mapUnique(err)
	}
	return user, org, nil
}

// mapUnique translates a PostgreSQL unique violation into domain.ErrConflict so
// the HTTP layer can avoid disclosing account existence.
func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

// UserByEmail returns the user and the stored hash (empty for OIDC-only accounts).
func (s *AccountStore) UserByEmail(ctx context.Context, email string) (domain.User, string, error) {
	row, err := s.q.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, "", domain.ErrNotFound
		}
		return domain.User{}, "", err
	}
	hash := ""
	if row.PasswordHash != nil {
		hash = *row.PasswordHash
	}
	return userFromRow(row), hash, nil
}

// MarkEmailVerified is idempotent.
func (s *AccountStore) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	return s.q.MarkEmailVerified(ctx, pgstore.MarkEmailVerifiedParams{
		ID: toUUID(userID), EmailVerifiedAt: toTS(at),
	})
}

// UpdatePassword replaces the password hash.
func (s *AccountStore) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return s.q.UpdatePassword(ctx, pgstore.UpdatePasswordParams{
		ID: toUUID(userID), PasswordHash: &passwordHash,
	})
}

// PrimaryMembership returns the org and role used for the session. The query
// runs under app.user_id so the memberships_self RLS policy applies.
func (s *AccountStore) PrimaryMembership(ctx context.Context, userID string) (string, string, error) {
	var orgID, role string
	err := db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.user_id', $1, true)", userID); err != nil {
			return err
		}
		row, err := s.q.WithTx(tx).PrimaryMembership(ctx, toUUID(userID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		orgID, role = uuidString(row.OrgID), row.Role
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return orgID, role, nil
}

// CreateVerification stores a hashed email verification token.
func (s *AccountStore) CreateVerification(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	return s.q.CreateVerification(ctx, pgstore.CreateVerificationParams{
		UserID: toUUID(userID), TokenHash: tokenHash, ExpiresAt: toTS(expiresAt),
	})
}

// ConsumeVerification marks the token used and returns the user id.
func (s *AccountStore) ConsumeVerification(ctx context.Context, tokenHash []byte, at time.Time) (string, error) {
	row, err := s.q.ConsumeVerification(ctx, pgstore.ConsumeVerificationParams{
		TokenHash: tokenHash, UsedAt: toTS(at),
	})
	if err != nil {
		return "", err
	}
	return uuidString(row), nil
}

// CreatePasswordReset stores a hashed reset token.
func (s *AccountStore) CreatePasswordReset(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time, _ string) error {
	return s.q.CreatePasswordReset(ctx, pgstore.CreatePasswordResetParams{
		UserID: toUUID(userID), TokenHash: tokenHash, ExpiresAt: toTS(expiresAt),
	})
}

// ConsumePasswordReset marks the token used and returns the user id.
func (s *AccountStore) ConsumePasswordReset(ctx context.Context, tokenHash []byte, at time.Time) (string, error) {
	row, err := s.q.ConsumePasswordReset(ctx, pgstore.ConsumePasswordResetParams{
		TokenHash: tokenHash, UsedAt: toTS(at),
	})
	if err != nil {
		return "", err
	}
	return uuidString(row), nil
}

// RecordAudit appends an audit record. Events without an org fall back to the
// actor's primary membership; if there is none the event is dropped (it cannot
// satisfy the NOT NULL org_id) and the caller logs a warning.
func (s *AccountStore) RecordAudit(ctx context.Context, ev auth.AuditEvent) error {
	orgID := ev.OrgID
	if orgID == "" && ev.ActorID != "" {
		var err error
		orgID, _, err = s.PrimaryMembership(ctx, ev.ActorID)
		if err != nil {
			return nil
		}
	}
	if orgID == "" {
		return nil
	}
	meta, err := json.Marshal(ev.Metadata)
	if err != nil {
		return err
	}
	actorKind := ev.Actor
	if actorKind == "" {
		actorKind = "system"
	}
	// The audit table is protected by RLS; the insert runs with the tenant set.
	return db.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", orgID); err != nil {
			return err
		}
		return s.q.WithTx(tx).InsertAudit(ctx, pgstore.InsertAuditParams{
			OrgID:     toUUID(orgID),
			ActorID:   toUUID(ev.ActorID),
			ActorKind: actorKind,
			Action:    ev.Action,
			Target:    ev.Target,
			Metadata:  meta,
		})
	})
}

func userFromRow(row pgstore.User) domain.User {
	u := domain.User{
		ID:        uuidString(row.ID),
		Email:     row.Email,
		Name:      row.Name,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.EmailVerifiedAt.Valid {
		u.EmailVerified = true
	}
	return u
}

// UserIDByIdentity resolves a user from an OIDC identity.
func (s *AccountStore) UserIDByIdentity(ctx context.Context, provider, subject string) (string, error) {
	id, err := s.q.UserIDByIdentity(ctx, pgstore.UserIDByIdentityParams{Provider: provider, Subject: subject})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return uuidString(id), nil
}

// LinkIdentity stores the provider subject for a user.
func (s *AccountStore) LinkIdentity(ctx context.Context, provider, subject, userID string) error {
	return s.q.CreateIdentity(ctx, pgstore.CreateIdentityParams{
		Provider: provider, Subject: subject, UserID: toUUID(userID),
	})
}

// CreateUser inserts a user without an org (invite acceptance).
func (s *AccountStore) CreateUser(ctx context.Context, email, name, passwordHash string, verifiedAt time.Time) (domain.User, error) {
	row, err := s.q.CreateUser(ctx, pgstore.CreateUserParams{
		Email: email, Name: name, PasswordHash: &passwordHash, EmailVerifiedAt: toTS(verifiedAt),
	})
	if err != nil {
		return domain.User{}, mapUnique(err)
	}
	return userFromRow(row), nil
}

// AddMembership adds a user to an org (invite acceptance).
func (s *AccountStore) AddMembership(ctx context.Context, orgID, userID, role string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).AddMembership(ctx, pgstore.AddMembershipParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Role: role,
		})
	})
}

// SetMembershipRole updates a non-owner membership role.
func (s *AccountStore) SetMembershipRole(ctx context.Context, userID, orgID, role string) error {
	return s.q.SetMembershipRole(ctx, pgstore.SetMembershipRoleParams{
		UserID: toUUID(userID), OrgID: toUUID(orgID), Role: role,
	})
}
