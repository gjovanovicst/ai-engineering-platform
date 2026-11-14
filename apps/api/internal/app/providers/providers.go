// Package providers stores per-org AI provider configuration. API keys are
// encrypted at rest and never returned by the API.
package providers

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Provider is one configured AI endpoint.
type Provider struct {
	ID        string
	OrgID     string
	Name      string
	BaseURL   string
	Model     string
	Kind      string
	KeyHint   string
	HasKey    bool
	Enabled   bool
	CreatedAt time.Time
}

// Stored is the persisted row including the encrypted key.
type Stored struct {
	Provider
	APIKeyEnc []byte
}

// Store persists provider configs.
type Store interface {
	Create(ctx context.Context, p Stored) (Stored, error)
	List(ctx context.Context, orgID string) ([]Stored, error)
	Get(ctx context.Context, orgID, id string) (Stored, error)
	Default(ctx context.Context, orgID string) (Stored, error)
	RotateKey(ctx context.Context, orgID, id string, enc []byte, hint string) (Stored, error)
	SetEnabled(ctx context.Context, orgID, id string, enabled bool) (Stored, error)
	Delete(ctx context.Context, orgID, id string) error
}

// Cipher encrypts API keys (AES-256-GCM).
type Cipher struct {
	key   []byte
	kms   KMS
	keyID string
}

// KeyedKMS wraps data keys under a specific key id (BYOK).
type KeyedKMS interface {
	WrapKey(ctx context.Context, keyID string, plain []byte) ([]byte, error)
	UnwrapKey(ctx context.Context, keyID string, wrapped []byte) ([]byte, error)
}

// ErrCryptoUnavailable means provider key storage is unavailable (e.g. BYOK
// disabled); callers should surface a validation error, not a 500.
var ErrCryptoUnavailable = errors.New("provider key storage is unavailable")

// KMS wraps data keys; implemented by the HTTP KMS adapter.
type KMS interface {
	Wrap(ctx context.Context, plain []byte) ([]byte, error)
	Unwrap(ctx context.Context, wrapped []byte) ([]byte, error)
}

// NewCipher parses a hex or base64 32-byte key. Empty means "no encryption
// configured": keys cannot be stored.
func NewCipher(encoded string) (*Cipher, error) { return NewCipherWithKMS(encoded, nil) }

// NewCipherWithKMS uses an external KMS for DEK wrapping (version 3 rows).
func NewCipherWithKMS(encoded string, kmsClient KMS) (*Cipher, error) {
	return NewCipherWithKMSKey(encoded, kmsClient, "")
}

// NewCipherWithKMSKey binds the cipher to a specific KMS key id (version 4).
func NewCipherWithKMSKey(encoded string, kmsClient KMS, keyID string) (*Cipher, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(encoded)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encoded)
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("providers: secrets key must be 32 bytes (hex or base64)")
	}
	return &Cipher{key: key, kms: kmsClient, keyID: keyID}, nil
}

// Encrypt returns an envelope: version || wrapped DEK || ciphertext.
// Each secret gets a fresh 32-byte data key, wrapped by the master key, which is
// the shape a KMS-backed master key needs.
func (c *Cipher) Encrypt(plain string) ([]byte, error) {
	return c.EncryptCtx(context.Background(), plain)
}

// EncryptCtx seals with a per-secret DEK; version 3 is KMS-wrapped.
func (c *Cipher) EncryptCtx(ctx context.Context, plain string) ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, err
	}
	version := byte(2)
	var wrapped []byte
	var err error
	if c.kms != nil && c.keyID != "" {
		keyed, ok := c.kms.(KeyedKMS)
		if !ok {
			return nil, errors.New("providers: KMS does not support key ids")
		}
		version = 4
		wrapped, err = keyed.WrapKey(ctx, c.keyID, dek)
	} else if c.kms != nil {
		version = 3
		wrapped, err = c.kms.Wrap(ctx, dek)
	} else {
		wrapped, err = c.seal(c.key, dek)
	}
	if err != nil {
		return nil, err
	}
	payload, err := c.seal(dek, []byte(plain))
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 3+len(wrapped)+len(payload))
	out = append(out, version)
	if version == 3 {
		if len(wrapped) > 65535 {
			return nil, errors.New("providers: wrapped key is too large")
		}
		size := uint16(len(wrapped)) //nolint:gosec // guarded <= 65535 above
		out = append(out, byte(size>>8), byte(size))
	}
	if version == 4 {
		if len(c.keyID) > 65535 || len(wrapped) > 65535 {
			return nil, errors.New("providers: key id or wrapped key is too large")
		}
		idSize := uint16(len(c.keyID)) //nolint:gosec // guarded above
		out = append(out, byte(idSize>>8), byte(idSize))
		out = append(out, c.keyID...)
		keySize := uint16(len(wrapped)) //nolint:gosec // guarded above
		out = append(out, byte(keySize>>8), byte(keySize))
	}
	out = append(out, wrapped...)
	out = append(out, payload...)
	return out, nil
}

func (c *Cipher) seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (c *Cipher) open(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("providers: ciphertext is too short")
	}
	nonce, body := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

// Decrypt supports both the envelope format and legacy nonce||ciphertext rows.
func (c *Cipher) Decrypt(data []byte) (string, error) {
	return c.DecryptCtx(context.Background(), data)
}

func (c *Cipher) decryptLegacy(data []byte) (string, error) {
	plain, err := c.open(c.key, data)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// DecryptCtx reads v3 (KMS), v2 (local envelope) and legacy rows.
func (c *Cipher) DecryptCtx(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 {
		return "", errors.New("providers: ciphertext is empty")
	}
	if data[0] == 4 {
		keyed, ok := c.kms.(KeyedKMS)
		if !ok {
			return c.decryptLegacy(data)
		}
		if len(data) < 5 {
			return "", errors.New("providers: envelope is truncated")
		}
		idSize := int(data[1])<<8 | int(data[2])
		if len(data) < 3+idSize+2 {
			return "", errors.New("providers: envelope is truncated")
		}
		keyID := string(data[3 : 3+idSize])
		size := int(data[3+idSize])<<8 | int(data[4+idSize])
		start := 5 + idSize
		if len(data) < start+size {
			return "", errors.New("providers: envelope is truncated")
		}
		dek, err := keyed.UnwrapKey(ctx, keyID, data[start:start+size])
		if err != nil {
			return "", err
		}
		plain, err := c.open(dek, data[start+size:])
		if err != nil {
			return c.decryptLegacy(data)
		}
		return string(plain), nil
	}
	if data[0] == 3 {
		if c.kms == nil {
			return c.decryptLegacy(data)
		}
		if len(data) < 3 {
			return "", errors.New("providers: envelope is truncated")
		}
		size := int(data[1])<<8 | int(data[2])
		if len(data) < 3+size {
			return "", errors.New("providers: envelope is truncated")
		}
		dek, err := c.kms.Unwrap(ctx, data[3:3+size])
		if err != nil {
			return "", err
		}
		plain, err := c.open(dek, data[3+size:])
		if err != nil {
			return c.decryptLegacy(data)
		}
		return string(plain), nil
	}
	if data[0] == 2 {
		// wrapped DEK is nonce(12)+key(32)+tag(16) = 60 bytes.
		if len(data) < 1+60 {
			return c.decryptLegacy(data)
		}
		dek, err := c.open(c.key, data[1:61])
		if err != nil {
			return "", err
		}
		plain, err := c.open(dek, data[61:])
		if err != nil {
			return c.decryptLegacy(data)
		}
		return string(plain), nil
	}
	plain, err := c.open(c.key, data)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// Service validates and stores providers.
type Service struct {
	store          Store
	cipher         *Cipher
	cipherResolver func(ctx context.Context, orgID string) (*Cipher, error)
	log            *slog.Logger
}

// WithCipherResolver allows a per-organisation cipher (BYOK).
func (s *Service) WithCipherResolver(resolver func(ctx context.Context, orgID string) (*Cipher, error)) *Service {
	s.cipherResolver = resolver
	return s
}

func (s *Service) cipherFor(ctx context.Context, orgID string) (*Cipher, error) {
	if s.cipherResolver != nil {
		return s.cipherResolver(ctx, orgID)
	}
	return s.cipher, nil
}

// New wires the service.
func New(store Store, cipher *Cipher, log *slog.Logger) (*Service, error) {
	if store == nil {
		return nil, errors.New("providers: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, cipher: cipher, log: log}, nil
}

// CreateInput is the API payload.
type CreateInput struct {
	OrgID   string
	Name    string
	BaseURL string
	Model   string
	Kind    string
	APIKey  string
}

// Create stores a provider; the key is encrypted and only a hint is kept.
func (s *Service) Create(ctx context.Context, in CreateInput) (Provider, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Provider{}, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	parsed, err := url.Parse(strings.TrimSpace(in.BaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Provider{}, fmt.Errorf("%w: base_url must be an absolute http(s) URL", domain.ErrValidation)
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "openai"
	}
	if kind != "openai" && kind != "anthropic" {
		return Provider{}, fmt.Errorf("%w: kind must be openai or anthropic", domain.ErrValidation)
	}
	stored := Stored{Provider: Provider{
		OrgID: in.OrgID, Name: name, BaseURL: parsed.String(), Model: strings.TrimSpace(in.Model),
		Kind: kind, Enabled: true,
	}}
	if key := strings.TrimSpace(in.APIKey); key != "" {
		cipher, cerr := s.cipherFor(ctx, in.OrgID)
		if cerr != nil {
			return Provider{}, cerr
		}
		if cipher == nil {
			return Provider{}, fmt.Errorf("%w: AIP_SECRETS_KEY is not configured; refusing to store a key", domain.ErrValidation)
		}
		enc, err := cipher.EncryptCtx(ctx, key)
		if err != nil {
			return Provider{}, err
		}
		stored.APIKeyEnc = enc
		stored.KeyHint = hint(key)
	}
	created, err := s.store.Create(ctx, stored)
	if err != nil {
		return Provider{}, err
	}
	return created.Provider, nil
}

// List returns providers without secrets.
func (s *Service) List(ctx context.Context, orgID string) ([]Provider, error) {
	rows, err := s.store.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Provider)
	}
	return out, nil
}

// Default returns the first enabled provider and its decrypted key.
func (s *Service) Default(ctx context.Context, orgID string) (Provider, string, error) {
	stored, err := s.store.Default(ctx, orgID)
	if err != nil {
		return Provider{}, "", err
	}
	if len(stored.APIKeyEnc) == 0 {
		return stored.Provider, "", nil
	}
	cipher, cerr := s.cipherFor(ctx, orgID)
	if cerr != nil {
		return Provider{}, "", cerr
	}
	if cipher == nil {
		return Provider{}, "", errors.New("providers: AIP_SECRETS_KEY is not configured")
	}
	key, err := cipher.DecryptCtx(ctx, stored.APIKeyEnc)
	if err != nil {
		return Provider{}, "", err
	}
	return stored.Provider, key, nil
}

// RotateKey replaces the stored key (old ciphertext is overwritten).
func (s *Service) RotateKey(ctx context.Context, orgID, id, key string) (Provider, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Provider{}, fmt.Errorf("%w: api_key is required", domain.ErrValidation)
	}
	cipher, cerr := s.cipherFor(ctx, orgID)
	if cerr != nil {
		return Provider{}, cerr
	}
	if cipher == nil {
		return Provider{}, fmt.Errorf("%w: AIP_SECRETS_KEY is not configured; refusing to store a key", domain.ErrValidation)
	}
	enc, err := cipher.EncryptCtx(ctx, key)
	if err != nil {
		return Provider{}, err
	}
	stored, err := s.store.RotateKey(ctx, orgID, id, enc, hint(key))
	if err != nil {
		return Provider{}, err
	}
	return stored.Provider, nil
}

// SetEnabled toggles a provider without touching the key.
func (s *Service) SetEnabled(ctx context.Context, orgID, id string, enabled bool) (Provider, error) {
	stored, err := s.store.SetEnabled(ctx, orgID, id, enabled)
	if err != nil {
		return Provider{}, err
	}
	return stored.Provider, nil
}

// Delete removes a provider and its key.
func (s *Service) Delete(ctx context.Context, orgID, id string) error {
	return s.store.Delete(ctx, orgID, id)
}

// hint keeps the last four characters so the UI can show which key is stored.
func hint(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return "…" + key[len(key)-4:]
}
