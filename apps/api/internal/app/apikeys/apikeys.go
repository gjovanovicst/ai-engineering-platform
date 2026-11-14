// Package apikeys implements service-account API keys.
package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Key is metadata for a service account key (never the secret).
type Key struct {
	ID         string
	OrgID      string
	Name       string
	Role       string
	Scopes     []string
	CreatedAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
}

// Store persists key hashes.
type Store interface {
	Create(ctx context.Context, orgID, name, role, hash string, scopes []string, expires *time.Time) (Key, error)
	ByHash(ctx context.Context, orgID, hash string) (Key, error)
	List(ctx context.Context, orgID string) ([]Key, error)
	Revoke(ctx context.Context, orgID, id string) error
	Touch(ctx context.Context, id string) error
}

// Service issues and authenticates keys.
type Service struct{ store Store }

// New wires the service.
func New(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("apikeys: store is required")
	}
	return &Service{store: store}, nil
}

var roles = map[string]bool{"admin": true, "member": true, "viewer": true}

// Create returns the plaintext token exactly once.
func (s *Service) Create(ctx context.Context, orgID, name, role string, ttl time.Duration) (Key, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return Key{}, "", fmt.Errorf("%w: name is required (max 120)", domain.ErrValidation)
	}
	if !roles[role] {
		return Key{}, "", fmt.Errorf("%w: role must be admin, member or viewer", domain.ErrValidation)
	}
	if ttl < 0 || ttl > 365*24*time.Hour {
		return Key{}, "", fmt.Errorf("%w: expiry must be between 1 day and 365 days (0 = never)", domain.ErrValidation)
	}
	var expires *time.Time
	if ttl > 0 {
		t := time.Now().UTC().Add(ttl)
		expires = &t
	}
	secret, err := random(32)
	if err != nil {
		return Key{}, "", err
	}
	sum := sha256.Sum256([]byte(secret))
	key, err := s.store.Create(ctx, orgID, name, role, hex.EncodeToString(sum[:]), []string{"api_key"}, expires)
	if err != nil {
		return Key{}, "", err
	}
	token := "aip." + base64.RawURLEncoding.EncodeToString([]byte(orgID)) + "." + secret
	return key, token, nil
}

// List returns org keys.
func (s *Service) List(ctx context.Context, orgID string) ([]Key, error) {
	return s.store.List(ctx, orgID)
}

// Revoke disables a key.
func (s *Service) Revoke(ctx context.Context, orgID, id string) error {
	return s.store.Revoke(ctx, orgID, id)
}

// Authenticate resolves a token to a key.
func (s *Service) Authenticate(ctx context.Context, token string) (Key, error) {
	orgID, secret, ok := ParseToken(token)
	if !ok {
		return Key{}, domain.ErrNotFound
	}
	sum := sha256.Sum256([]byte(secret))
	key, err := s.store.ByHash(ctx, orgID, hex.EncodeToString(sum[:]))
	if err != nil {
		return Key{}, err
	}
	if key.RevokedAt != nil || (key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt)) {
		return Key{}, domain.ErrNotFound
	}
	_ = s.store.Touch(ctx, key.ID)
	return key, nil
}

// ParseToken splits aip.<base64url org>.<secret>.
func ParseToken(token string) (orgID, secret string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "aip" {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(raw) == 0 || parts[2] == "" {
		return "", "", false
	}
	return string(raw), parts[2], true
}

func random(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
