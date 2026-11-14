package domain

import (
	"net/mail"
	"strings"
	"time"
)

// User is a person. Users are global (they may belong to several orgs through
// memberships); the domain does not know about org scope here.
type User struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
	CreatedAt     time.Time
}

// NormalizeEmail lowercases and trims an address, then validates it.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", invalid("email", "must not be empty")
	}
	if len(email) > 254 {
		return "", invalid("email", "must be at most 254 characters")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", invalid("email", "is not a valid address")
	}
	return email, nil
}

// NewUser validates a new account. Password hashing is an application concern;
// the domain only stores an opaque hash when one exists.
func NewUser(email, name string) (*User, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	u := &User{Email: normalized, Name: strings.TrimSpace(name)}
	if err := u.Validate(); err != nil {
		return nil, err
	}
	return u, nil
}

// Validate enforces the user invariants.
func (u *User) Validate() error {
	if _, err := NormalizeEmail(u.Email); err != nil {
		return err
	}
	if strings.TrimSpace(u.Name) == "" {
		return invalid("name", "must not be empty")
	}
	if len(u.Name) > 80 {
		return invalid("name", "must be at most 80 characters")
	}
	return nil
}
