// Package invites lets admins add teammates to an organisation.
package invites

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Invite is a pending membership invitation.
type Invite struct {
	ID         string
	OrgID      string
	Email      string
	Role       string
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	CreatedAt  time.Time
}

// Store persists invites and the membership created on acceptance.
type Store interface {
	Create(ctx context.Context, orgID, email, role string, tokenHash []byte, expires time.Time, createdBy string) (Invite, error)
	List(ctx context.Context, orgID string, limit int) ([]Invite, error)
	OrgForToken(ctx context.Context, tokenHash []byte) (string, error)
	ByToken(ctx context.Context, orgID string, tokenHash []byte) (Invite, error)
	Accept(ctx context.Context, orgID, id string) (Invite, error)
	AddMembership(ctx context.Context, orgID, userID, role string) error
	EmailForUser(ctx context.Context, userID string) (string, error)
}

// AccountCreator provisions the invited user and signs them in.
type AccountCreator interface {
	CreateInvitedUser(ctx context.Context, email, name, password, orgID, role string) (auth.TokenPair, error)
}

// Mailer delivers the invitation link.
type Mailer interface {
	SendInvite(ctx context.Context, to, link string) error
}

// Service creates and accepts invites.
type Service struct {
	store    Store
	mailer   Mailer
	accounts AccountCreator
	webURL   string
	now      func() time.Time
	log      *slog.Logger
}

// New wires the service.
func New(store Store, mailer Mailer, accounts AccountCreator, webURL string, log *slog.Logger) (*Service, error) {
	if store == nil {
		return nil, errors.New("invites: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{
		store: store, mailer: mailer, accounts: accounts, webURL: webURL,
		now: func() time.Time { return time.Now().UTC() }, log: log,
	}, nil
}

// CreateInput is the invite payload.
type CreateInput struct {
	OrgID     string
	Email     string
	Role      string
	CreatedBy string
}

// Created is returned only once, with the raw token.
type Created struct {
	Invite Invite
	Token  string
}

// Create issues an invite valid for 7 days.
func (s *Service) Create(ctx context.Context, in CreateInput) (Created, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return Created{}, fmt.Errorf("%w: email is invalid", domain.ErrValidation)
	}
	if !oneOf(in.Role, "owner", "admin", "member", "viewer") {
		return Created{}, fmt.Errorf("%w: role is invalid", domain.ErrValidation)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return Created{}, err
	}
	raw := hex.EncodeToString(token)
	hash := sha256.Sum256([]byte(raw))
	expires := s.now().Add(7 * 24 * time.Hour)

	invite, err := s.store.Create(ctx, in.OrgID, email, in.Role, hash[:], expires, in.CreatedBy)
	if err != nil {
		return Created{}, err
	}
	if s.mailer != nil {
		link := fmt.Sprintf("%s/invites/accept?org=%s&token=%s", strings.TrimRight(s.webURL, "/"), in.OrgID, raw)
		if merr := s.mailer.SendInvite(ctx, email, link); merr != nil {
			// The invite exists; a mail failure must not lose it.
			s.log.Warn("invites: send email", "err", merr, "invite_id", invite.ID)
		}
	}
	return Created{Invite: invite, Token: raw}, nil
}

// List returns pending and accepted invites.
func (s *Service) List(ctx context.Context, orgID string, limit int) ([]Invite, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.List(ctx, orgID, limit)
}

// RegisterAccept creates the invited account and signs it in (no prior session).
func (s *Service) RegisterAccept(ctx context.Context, token, name, password string) (auth.TokenPair, error) {
	invite, orgID, err := s.pending(ctx, token)
	if err != nil {
		return auth.TokenPair{}, err
	}
	if s.accounts == nil {
		return auth.TokenPair{}, fmt.Errorf("invites: account creation is not configured")
	}
	pair, err := s.accounts.CreateInvitedUser(ctx, invite.Email, name, password, orgID, invite.Role)
	if err != nil {
		return auth.TokenPair{}, err
	}
	if _, err := s.store.Accept(ctx, orgID, invite.ID); err != nil {
		return auth.TokenPair{}, err
	}
	return pair, nil
}

// pending resolves and validates an invite token.
func (s *Service) pending(ctx context.Context, token string) (Invite, string, error) {
	raw := strings.TrimSpace(token)
	if raw == "" {
		return Invite{}, "", fmt.Errorf("%w: token is required", domain.ErrValidation)
	}
	sum := sha256.Sum256([]byte(raw))
	orgID, err := s.store.OrgForToken(ctx, sum[:])
	if err != nil {
		return Invite{}, "", err
	}
	invite, err := s.store.ByToken(ctx, orgID, sum[:])
	if err != nil {
		return Invite{}, "", err
	}
	if invite.AcceptedAt != nil {
		return Invite{}, "", fmt.Errorf("%w: the invite was already accepted", domain.ErrConflict)
	}
	if s.now().After(invite.ExpiresAt) {
		return Invite{}, "", fmt.Errorf("%w: the invite expired", domain.ErrValidation)
	}
	return invite, orgID, nil
}

// Accept adds the signed-in user to the org if the token, email and expiry match.
func (s *Service) Accept(ctx context.Context, orgID, token, userID string) (Invite, error) {
	raw := strings.TrimSpace(token)
	if raw == "" {
		return Invite{}, fmt.Errorf("%w: token is required", domain.ErrValidation)
	}
	sum := sha256.Sum256([]byte(raw))

	inviteOrg, err := s.store.OrgForToken(ctx, sum[:])
	if err != nil {
		return Invite{}, err
	}
	if inviteOrg != orgID {
		return Invite{}, domain.ErrNotFound
	}
	invite, err := s.store.ByToken(ctx, orgID, sum[:])
	if err != nil {
		return Invite{}, err
	}
	if invite.AcceptedAt != nil {
		return Invite{}, fmt.Errorf("%w: the invite was already accepted", domain.ErrConflict)
	}
	if s.now().After(invite.ExpiresAt) {
		return Invite{}, fmt.Errorf("%w: the invite expired", domain.ErrValidation)
	}
	userEmail, err := s.store.EmailForUser(ctx, userID)
	if err != nil {
		return Invite{}, err
	}
	if !strings.EqualFold(invite.Email, userEmail) {
		return Invite{}, fmt.Errorf("%w: the invite belongs to another email", domain.ErrValidation)
	}
	if err := s.store.AddMembership(ctx, orgID, userID, invite.Role); err != nil {
		return Invite{}, err
	}
	return s.store.Accept(ctx, orgID, invite.ID)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
