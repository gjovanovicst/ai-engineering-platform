// Package scim provides read-only SCIM 2.0 user provisioning.
package scim

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/invites"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Token is an issued SCIM bearer token (plaintext shown once).
type Token struct {
	ID        string
	Name      string
	CreatedAt time.Time
	RevokedAt *time.Time
}

// User is a SCIM user projection.
type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
	Active    bool
}

// Store persists SCIM tokens and reads users.
type Store interface {
	CreateToken(ctx context.Context, orgID, name, hash string) (Token, error)
	ListTokens(ctx context.Context, orgID string) ([]Token, error)
	RevokeToken(ctx context.Context, orgID, id string) error
	TokenOrg(ctx context.Context, orgID, hash string) (string, error)
	Users(ctx context.Context, orgID string) ([]User, error)
	OwnerID(ctx context.Context, orgID string) (string, error)
	MembershipRole(ctx context.Context, orgID, userID string) (string, error)
	RemoveMembership(ctx context.Context, orgID, userID string) error
	UpdateMembershipRole(ctx context.Context, orgID, userID, role string) error
	UpdateUserName(ctx context.Context, orgID, userID, name string) error
	UpdateUserEmail(ctx context.Context, orgID, userID, email string) error
}

// Inviter provisions an invited account (SCIM create is invite-backed).
type Inviter interface {
	Create(ctx context.Context, in invites.CreateInput) (invites.Created, error)
}

// Service issues and verifies SCIM tokens.
type Service struct {
	store   Store
	inviter Inviter
}

// New wires the service.
func New(store Store, inviter Inviter) *Service { return &Service{store: store, inviter: inviter} }

func random(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	return buf, err
}

// CreateToken returns the token once and stores only its hash.
func (s *Service) CreateToken(ctx context.Context, orgID, name string) (Token, string, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 80 {
		return Token{}, "", fmt.Errorf("%w: token name must be 2..80 characters", domain.ErrValidation)
	}
	secret, err := random(32)
	if err != nil {
		return Token{}, "", err
	}
	raw := "scim." + base64.RawURLEncoding.EncodeToString([]byte(orgID)) + "." + base64.RawURLEncoding.EncodeToString(secret)
	token, err := s.store.CreateToken(ctx, orgID, name, hash(raw))
	if err != nil {
		return Token{}, "", err
	}
	return token, raw, nil
}

// ListTokens returns issued tokens.
func (s *Service) ListTokens(ctx context.Context, orgID string) ([]Token, error) {
	return s.store.ListTokens(ctx, orgID)
}

// RevokeToken disables a token.
func (s *Service) RevokeToken(ctx context.Context, orgID, id string) error {
	return s.store.RevokeToken(ctx, orgID, id)
}

// Authenticate resolves a raw token to its org.
func (s *Service) Authenticate(ctx context.Context, raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || parts[0] != "scim" {
		return "", domain.ErrNotFound
	}
	orgBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(orgBytes) == 0 {
		return "", domain.ErrNotFound
	}
	orgID := string(orgBytes)
	return s.store.TokenOrg(ctx, orgID, hash(raw))
}

// CreateUser provisions a pending user through the invite flow, so SCIM create
// never bypasses email verification/password setup.
func (s *Service) CreateUser(ctx context.Context, orgID, email, name string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return User{}, fmt.Errorf("%w: userName must be an email", domain.ErrValidation)
	}
	if s.inviter == nil {
		return User{}, fmt.Errorf("%w: invites are not configured", domain.ErrValidation)
	}
	owner, err := s.store.OwnerID(ctx, orgID)
	if err != nil {
		return User{}, err
	}
	created, err := s.inviter.Create(ctx, invites.CreateInput{OrgID: orgID, Email: email, Role: "member", CreatedBy: owner})
	if err != nil {
		return User{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = email
	}
	return User{ID: created.Invite.ID, Email: created.Invite.Email, Name: name, CreatedAt: created.Invite.CreatedAt, Active: false}, nil
}

// Users lists users for the org.
func (s *Service) Users(ctx context.Context, orgID string) ([]User, error) {
	return s.store.Users(ctx, orgID)
}

func hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// DeactivateUser removes a non-owner membership (SCIM active=false).
func (s *Service) DeactivateUser(ctx context.Context, orgID, userID string) error {
	role, err := s.store.MembershipRole(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if role == "owner" {
		return fmt.Errorf("%w: cannot deactivate the org owner via SCIM", domain.ErrValidation)
	}
	return s.store.RemoveMembership(ctx, orgID, userID)
}

var roles = map[string]bool{"admin": true, "member": true, "viewer": true}

// SetRole changes a non-owner membership role (owner is managed in the product).
func (s *Service) SetRole(ctx context.Context, orgID, userID, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if !roles[role] {
		return fmt.Errorf("%w: role must be admin, member or viewer", domain.ErrValidation)
	}
	current, err := s.store.MembershipRole(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if current == "owner" {
		return fmt.Errorf("%w: cannot change the org owner role via SCIM", domain.ErrValidation)
	}
	return s.store.UpdateMembershipRole(ctx, orgID, userID, role)
}

// SetName changes a member's display name.
func (s *Service) SetName(ctx context.Context, orgID, userID, name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 120 {
		return fmt.Errorf("%w: name must be 1..120 characters", domain.ErrValidation)
	}
	return s.store.UpdateUserName(ctx, orgID, userID, name)
}

// SetEmail changes a member's login email.
func (s *Service) SetEmail(ctx context.Context, orgID, userID, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return fmt.Errorf("%w: email is invalid", domain.ErrValidation)
	}
	return s.store.UpdateUserEmail(ctx, orgID, userID, email)
}
