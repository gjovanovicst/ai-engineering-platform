package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/invites"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// InviteStore persists invites.
type InviteStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewInviteStore wires the store.
func NewInviteStore(pool *pgxpool.Pool) *InviteStore {
	return &InviteStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts an invite.
func (s *InviteStore) Create(ctx context.Context, orgID, email, role string, tokenHash []byte, expires time.Time, createdBy string) (invites.Invite, error) {
	var out invites.Invite
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateInvite(ctx, pgstore.CreateInviteParams{
			OrgID: toUUID(orgID), Email: email, Role: role, TokenHash: tokenHash,
			ExpiresAt: toTS(expires), CreatedBy: toUUID(createdBy),
		})
		if err != nil {
			return err
		}
		out = inviteFromRow(row)
		return nil
	})
	return out, err
}

// List returns the org invites.
func (s *InviteStore) List(ctx context.Context, orgID string, limit int) ([]invites.Invite, error) {
	var out []invites.Invite
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListInvites(ctx, pgstore.ListInvitesParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]invites.Invite, 0, len(rows))
		for _, row := range rows {
			out = append(out, inviteFromRow(row))
		}
		return nil
	})
	return out, err
}

// OrgForToken resolves the invite's org without a session.
func (s *InviteStore) OrgForToken(ctx context.Context, tokenHash []byte) (string, error) {
	orgID, err := s.q.InviteOrg(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return uuidString(orgID), nil
}

// ByToken returns the invite for a token inside the org.
func (s *InviteStore) ByToken(ctx context.Context, orgID string, tokenHash []byte) (invites.Invite, error) {
	var out invites.Invite
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetInviteByToken(ctx, pgstore.GetInviteByTokenParams{
			OrgID: toUUID(orgID), TokenHash: tokenHash,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = inviteFromRow(row)
		return nil
	})
	return out, err
}

// Accept marks the invite as used.
func (s *InviteStore) Accept(ctx context.Context, orgID, id string) (invites.Invite, error) {
	var out invites.Invite
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).AcceptInvite(ctx, pgstore.AcceptInviteParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrConflict
			}
			return err
		}
		out = inviteFromRow(row)
		return nil
	})
	return out, err
}

// AddMembership adds the accepting user to the org.
func (s *InviteStore) AddMembership(ctx context.Context, orgID, userID, role string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).AddMembership(ctx, pgstore.AddMembershipParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Role: role,
		})
	})
}

// EmailForUser returns the user's email (the users table is global).
func (s *InviteStore) EmailForUser(ctx context.Context, userID string) (string, error) {
	email, err := s.q.UserEmail(ctx, toUUID(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return email, nil
}

func inviteFromRow(row pgstore.Invite) invites.Invite {
	inv := invites.Invite{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), Email: row.Email,
		Role: row.Role, ExpiresAt: row.ExpiresAt.Time, CreatedAt: row.CreatedAt.Time,
	}
	if row.AcceptedAt.Valid {
		v := row.AcceptedAt.Time
		inv.AcceptedAt = &v
	}
	return inv
}
