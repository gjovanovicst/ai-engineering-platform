package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Platform identifies the client that owns a session (seam S1 in
// docs/roadmap-later.md).
type Platform string

const (
	PlatformWeb    Platform = "web"
	PlatformMobile Platform = "mobile"
	PlatformCLI    Platform = "cli"
)

// Valid reports whether p is a known platform.
func (p Platform) Valid() bool {
	return p == PlatformWeb || p == PlatformMobile || p == PlatformCLI
}

// Session is one refresh-token family. Only the SHA-256 of the token is stored.
type Session struct {
	ID          string
	UserID      string
	OrgID       string
	Role        string
	RefreshHash []byte
	Platform    Platform
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// SessionStore is the persistence port.
type SessionStore interface {
	Create(ctx context.Context, s Session) error
	// ByRefreshHash returns the session for a hashed refresh token, including
	// revoked ones (reuse detection needs them).
	ByRefreshHash(ctx context.Context, hash []byte) (Session, error)
	// Rotate revokes oldID and inserts next in one transaction.
	Rotate(ctx context.Context, oldID string, next Session, at time.Time) error
	RevokeSession(ctx context.Context, id, reason string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID, reason string, at time.Time) error
	Touch(ctx context.Context, id string, at time.Time) error
}

// TokenPair is what the API returns (and what the web BFF puts in cookies).
type TokenPair struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int64
	RefreshExpiresIn int64
}

// IssueRequest describes a new session.
type IssueRequest struct {
	UserID   string
	OrgID    string
	Role     string
	Scopes   []string
	Platform Platform
}

// Service issues and rotates tokens. It holds no state beyond its dependencies.
type Service struct {
	store      SessionStore
	issuer     *Issuer
	refreshTTL time.Duration
	idleTTL    time.Duration
	now        func() time.Time
}

// NewService wires the token service.
// WithIdleTTL ends a session when the refresh token is unused for this long.
func (s *Service) WithIdleTTL(idle time.Duration) *Service {
	s.idleTTL = idle
	return s
}

func NewService(store SessionStore, issuer *Issuer, refreshTTL time.Duration) (*Service, error) {
	if store == nil || issuer == nil {
		return nil, fmt.Errorf("auth: store and issuer are required")
	}
	if refreshTTL <= 0 {
		return nil, fmt.Errorf("auth: refresh ttl must be positive")
	}
	return &Service{store: store, issuer: issuer, refreshTTL: refreshTTL, now: time.Now}, nil
}

// Issue creates a session and returns the first token pair.
func (s *Service) Issue(ctx context.Context, req IssueRequest) (TokenPair, error) {
	if req.UserID == "" {
		return TokenPair{}, fmt.Errorf("auth: user id is required")
	}
	if req.Platform == "" {
		req.Platform = PlatformWeb
	}
	if !req.Platform.Valid() {
		return TokenPair{}, ErrInvalidPlatform
	}

	now := s.now().UTC()
	raw, hash, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	sessionID, err := newUUID()
	if err != nil {
		return TokenPair{}, err
	}
	session := Session{
		ID:          sessionID,
		UserID:      req.UserID,
		OrgID:       req.OrgID,
		Role:        req.Role,
		RefreshHash: hash,
		Platform:    req.Platform,
		ExpiresAt:   now.Add(s.refreshTTL),
		CreatedAt:   now,
	}
	if err := s.store.Create(ctx, session); err != nil {
		return TokenPair{}, fmt.Errorf("auth: create session: %w", err)
	}

	access, expires, err := s.issuer.Issue(req.UserID, Claims{
		OrgID:     req.OrgID,
		Role:      req.Role,
		SessionID: session.ID,
		Scopes:    req.Scopes,
	})
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		RefreshToken:     raw,
		ExpiresIn:        int64(time.Until(expires).Seconds()),
		RefreshExpiresIn: int64(s.refreshTTL.Seconds()),
	}, nil
}

// Refresh rotates a refresh token. Presenting an already rotated token revokes
// every session of that user (reuse detection, docs/12-auth.md §4.2).
func (s *Service) Refresh(ctx context.Context, rawRefresh string, platform Platform) (TokenPair, error) {
	hash := hashToken(rawRefresh)
	session, err := s.store.ByRefreshHash(ctx, hash)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}

	now := s.now().UTC()
	if session.RevokedAt != nil {
		_ = s.store.RevokeAllForUser(ctx, session.UserID, "refresh token reuse", now)
		return TokenPair{}, ErrRefreshReuse
	}
	if now.After(session.ExpiresAt) {
		_ = s.store.RevokeAllForUser(ctx, session.UserID, "session expired", now)
		return TokenPair{}, ErrSessionExpired
	}
	if s.idleTTL > 0 && session.LastUsedAt != nil && now.Sub(*session.LastUsedAt) > s.idleTTL {
		_ = s.store.RevokeSession(ctx, session.ID, "idle timeout", now)
		return TokenPair{}, ErrSessionExpired
	}
	if platform == "" {
		platform = session.Platform
	}

	rawNext, hashNext, err := newRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	nextID, err := newUUID()
	if err != nil {
		return TokenPair{}, err
	}
	next := Session{
		ID:          nextID,
		UserID:      session.UserID,
		OrgID:       session.OrgID,
		Role:        session.Role,
		RefreshHash: hashNext,
		Platform:    platform,
		ExpiresAt:   session.ExpiresAt, // absolute lifetime is not extended forever
		CreatedAt:   now,
	}
	if err := s.store.Rotate(ctx, session.ID, next, now); err != nil {
		return TokenPair{}, fmt.Errorf("auth: rotate session: %w", err)
	}

	access, expires, err := s.issuer.Issue(session.UserID, Claims{
		OrgID:     next.OrgID,
		Role:      next.Role,
		SessionID: next.ID,
	})
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		RefreshToken:     rawNext,
		ExpiresIn:        int64(time.Until(expires).Seconds()),
		RefreshExpiresIn: int64(time.Until(next.ExpiresAt).Seconds()),
	}, nil
}

// RevokeAllForUser ends every active session of a user.
func (s *Service) RevokeAllForUser(ctx context.Context, userID, reason string) error {
	return s.store.RevokeAllForUser(ctx, userID, reason, s.now().UTC())
}

// Revoke ends one session (logout).
func (s *Service) Revoke(ctx context.Context, sessionID, reason string) error {
	return s.store.RevokeSession(ctx, sessionID, reason, s.now().UTC())
}

func newRefreshToken() (raw string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("auth: refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth: entropy source: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:]), nil
}
