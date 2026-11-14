package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// IssuerAIP is the `iss` claim value.
	IssuerAIP = "aip"
	// AudienceAPI is the `aud` claim value.
	AudienceAPI = "aip-api"
)

// Claims is the access token payload (docs/12-auth.md §4.1).
type Claims struct {
	OrgID     string   `json:"org,omitempty"`
	Role      string   `json:"role,omitempty"`
	SessionID string   `json:"sid"`
	Scopes    []string `json:"scopes,omitempty"`
	jwt.RegisteredClaims
}

// Issuer signs and verifies access tokens with Ed25519 (EdDSA).
type Issuer struct {
	key      ed25519.PrivateKey
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

// NewIssuer parses a PEM-encoded Ed25519 private key.
func NewIssuer(pemKey []byte, ttl time.Duration) (*Issuer, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("auth: access token ttl must be positive")
	}
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("auth: jwt key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse jwt key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("auth: jwt key must be Ed25519")
	}
	return &Issuer{key: key, issuer: IssuerAIP, audience: AudienceAPI, ttl: ttl, now: time.Now}, nil
}

// Issue creates an access token for the user and returns its expiry.
func (i *Issuer) Issue(userID string, c Claims) (string, time.Time, error) {
	if userID == "" {
		return "", time.Time{}, fmt.Errorf("auth: user id is required")
	}
	now := i.now().UTC()
	expires := now.Add(i.ttl)
	c.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    i.issuer,
		Subject:   userID,
		Audience:  jwt.ClaimStrings{i.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expires),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(i.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign access token: %w", err)
	}
	return signed, expires, nil
}

// Parse validates signature, algorithm, issuer, audience and expiry.
func (i *Issuer) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		return i.key.Public(), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(i.issuer),
		jwt.WithAudience(i.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return claims, nil
}

// PublicKeyPEM returns the public key so that other services can verify
// tokens without the private material.
func (i *Issuer) PublicKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(i.key.Public())
	if err != nil {
		return nil, fmt.Errorf("auth: marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}
