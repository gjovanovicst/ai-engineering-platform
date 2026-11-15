// Package repocreds stores per-repository git credentials. A repo credential
// takes precedence over the GitHub App installation token and the global
// provider token, so one project can use a fine-grained PAT scoped to its own
// repository while the rest of the organisation keeps the shared credential.
package repocreds

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Cipher seals and opens credentials at rest (implemented by providers.Cipher,
// which is keyed by AIP_SECRETS_KEY).
type Cipher interface {
	Encrypt(string) ([]byte, error)
	Decrypt([]byte) (string, error)
}

// Record is a stored credential; Ciphertext is the sealed token.
type Record struct {
	RepoID     string
	Ciphertext []byte
	Hint       string
	CreatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Store persists one credential per repository.
type Store interface {
	Upsert(ctx context.Context, orgID, repoID, actorID, action string, rec Record) (Record, error)
	Get(ctx context.Context, orgID, repoID string) (Record, error)
	Delete(ctx context.Context, orgID, repoID, actorID, action string) error
}

// Status is the API-facing view. It never carries the token.
type Status struct {
	Configured bool
	Hint       string
	UpdatedAt  *time.Time
}

// Service stores, removes and resolves repository credentials.
type Service struct {
	store  Store
	cipher Cipher
}

// New wires the service. A nil cipher means AIP_SECRETS_KEY is not configured:
// reads keep working (no credential can exist), writes fail with a validation
// error instead of storing a plaintext token.
func New(store Store, cipher Cipher) (*Service, error) {
	if store == nil {
		return nil, errors.New("repocreds: store is required")
	}
	return &Service{store: store, cipher: cipher}, nil
}

const (
	// maxTokenLen bounds what a user can paste; PATs are far shorter.
	maxTokenLen = 4096
	minTokenLen = 8
	// hintSuffix is how many trailing characters are shown back to the user.
	hintSuffix = 4
)

// Set validates a token, seals it and stores it for one repository.
func (s *Service) Set(ctx context.Context, orgID, repoID, actorID, token string) (Status, error) {
	token = strings.TrimSpace(token)
	if err := validate(token); err != nil {
		return Status{}, err
	}
	if s.cipher == nil {
		return Status{}, fmt.Errorf("%w: AIP_SECRETS_KEY is required to store a repository credential", domain.ErrValidation)
	}
	sealed, err := s.cipher.Encrypt(token)
	if err != nil {
		return Status{}, fmt.Errorf("repocreds: encrypt: %w", err)
	}
	rec, err := s.store.Upsert(ctx, orgID, repoID, actorID, "repo.credential.set", Record{
		RepoID: repoID, Ciphertext: sealed, Hint: Hint(token), CreatedBy: actorID,
	})
	if err != nil {
		return Status{}, fmt.Errorf("repocreds: upsert: %w", err)
	}
	return status(rec), nil
}

// Delete removes the credential. Removing an absent credential is a no-op.
func (s *Service) Delete(ctx context.Context, orgID, repoID, actorID string) error {
	if err := s.store.Delete(ctx, orgID, repoID, actorID, "repo.credential.deleted"); err != nil {
		return fmt.Errorf("repocreds: delete: %w", err)
	}
	return nil
}

// Status reports whether a credential exists without decrypting it.
func (s *Service) Status(ctx context.Context, orgID, repoID string) (Status, error) {
	rec, err := s.store.Get(ctx, orgID, repoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Status{}, nil
		}
		return Status{}, fmt.Errorf("repocreds: get: %w", err)
	}
	return status(rec), nil
}

// Token returns the decrypted credential, or "" when none is configured.
func (s *Service) Token(ctx context.Context, orgID, repoID string) (string, error) {
	if s.cipher == nil {
		return "", nil
	}
	rec, err := s.store.Get(ctx, orgID, repoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("repocreds: get: %w", err)
	}
	token, err := s.cipher.Decrypt(rec.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("repocreds: decrypt: %w", err)
	}
	return token, nil
}

// Hint is the only part of a token that is stored in clear: its last four
// characters, so a human can tell which credential is configured.
func Hint(token string) string {
	if len(token) <= hintSuffix {
		return "…"
	}
	return "…" + token[len(token)-hintSuffix:]
}

func status(rec Record) Status {
	updated := rec.UpdatedAt
	return Status{Configured: true, Hint: rec.Hint, UpdatedAt: &updated}
}

func validate(token string) error {
	switch {
	case token == "":
		return fmt.Errorf("%w: token is required", domain.ErrValidation)
	case len(token) < minTokenLen:
		return fmt.Errorf("%w: token is too short", domain.ErrValidation)
	case len(token) > maxTokenLen:
		return fmt.Errorf("%w: token is too long (max %d bytes)", domain.ErrValidation, maxTokenLen)
	}
	for _, r := range token {
		if unicode.IsSpace(r) {
			return fmt.Errorf("%w: token must not contain whitespace", domain.ErrValidation)
		}
	}
	return nil
}
