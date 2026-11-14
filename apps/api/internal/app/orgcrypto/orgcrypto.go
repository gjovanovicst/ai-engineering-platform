// Package orgcrypto stores BYOK KMS configuration per organisation.
package orgcrypto

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Config is a BYOK KMS binding.
type Config struct {
	KMSURL    string
	KeyID     string
	TokenEnc  []byte
	Status    string
	UpdatedAt time.Time
}

// Cipher encrypts the KMS token at rest.
type Cipher interface {
	Encrypt(plain string) ([]byte, error)
	Decrypt(data []byte) (string, error)
}

// Store persists org crypto config.
type Store interface {
	Get(ctx context.Context, orgID string) (Config, error)
	Upsert(ctx context.Context, orgID, kmsURL, keyID string, tokenEnc []byte) (Config, error)
	Disable(ctx context.Context, orgID string) error
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service exposes BYOK configuration use cases.
type Service struct {
	store  Store
	cipher Cipher
}

// New wires the service; cipher encrypts the optional KMS token.
func New(store Store, cipher Cipher) *Service { return &Service{store: store, cipher: cipher} }

// Get returns the config or ErrNotFound.
func (s *Service) Get(ctx context.Context, orgID string) (Config, error) {
	return s.store.Get(ctx, orgID)
}

// Set validates and enables a BYOK KMS binding.
func (s *Service) Set(ctx context.Context, orgID, actorID, kmsURL, keyID, token string) (Config, error) {
	kmsURL = strings.TrimSpace(kmsURL)
	keyID = strings.TrimSpace(keyID)
	parsed, err := url.Parse(kmsURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return Config{}, fmt.Errorf("%w: kms_url must be an http(s) URL", domain.ErrValidation)
	}
	if keyID == "" || len(keyID) > 200 {
		return Config{}, fmt.Errorf("%w: key_id is required (max 200)", domain.ErrValidation)
	}
	tokenEnc := []byte{}
	if token = strings.TrimSpace(token); token != "" {
		if s.cipher == nil {
			return Config{}, fmt.Errorf("%w: AIP_SECRETS_KEY is required to store a KMS token", domain.ErrValidation)
		}
		var terr error
		tokenEnc, terr = s.cipher.Encrypt(token)
		if terr != nil {
			return Config{}, terr
		}
	}
	cfg, err := s.store.Upsert(ctx, orgID, kmsURL, keyID, tokenEnc)
	if err != nil {
		return Config{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "org.byok.enabled", orgID, []byte(`{}`))
	return cfg, nil
}

// Disable stops using the org KMS; existing v4 rows become unreadable.
func (s *Service) Disable(ctx context.Context, orgID, actorID string) error {
	if err := s.store.Disable(ctx, orgID); err != nil {
		return err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "org.byok.disabled", orgID, []byte(`{}`))
	return nil
}

// ErrNotConfigured is returned when an org has no BYOK config.
var ErrNotConfigured = errors.New("orgcrypto: not configured")
