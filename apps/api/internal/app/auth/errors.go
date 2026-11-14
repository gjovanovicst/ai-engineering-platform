// Package auth implements credentials, tokens and sessions. It is pure Go:
// persistence, HTTP and providers are injected through interfaces.
package auth

import "errors"

var (
	// ErrInvalidCredentials is returned when the password does not match.
	// Callers must never reveal whether the account exists.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidToken covers malformed, expired or unknown tokens.
	ErrInvalidToken = errors.New("invalid token")
	// ErrRefreshReuse means a rotated refresh token was presented again. The
	// whole session family is revoked when this happens.
	ErrRefreshReuse = errors.New("refresh token reuse detected")
	// ErrSessionExpired means the refresh token is past its absolute lifetime.
	ErrSessionExpired = errors.New("session expired")
	// ErrWeakPassword is returned by ValidatePassword.
	ErrWeakPassword = errors.New("password does not meet the policy")
	// ErrInvalidPlatform is returned for unknown client platforms.
	ErrInvalidPlatform = errors.New("invalid platform")
)
