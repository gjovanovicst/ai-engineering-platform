package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the argon2id cost parameters. They are stored inside the hash
// (PHC string) so that parameters can be raised later without breaking old
// hashes: VerifyPassword reports whether a rehash is needed.
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultParams follow docs/12-auth.md: t=3, m=64 MiB, p=2, 16-byte salt, 32-byte key.
func DefaultParams() Params {
	return Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLen: 16, KeyLen: 32}
}

// HashPassword returns a PHC-formatted argon2id hash.
func HashPassword(password string, p Params) (string, error) {
	if password == "" {
		return "", ErrWeakPassword
	}
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)

	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyResult reports the outcome of a password check.
type VerifyResult struct {
	Valid       bool
	NeedsRehash bool
}

// VerifyPassword checks a password against a PHC hash and reports whether the
// stored parameters are weaker than the current defaults.
func VerifyPassword(password, encoded string) (VerifyResult, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return VerifyResult{}, err
	}
	// len(want) is validated by decodeHash (max 1024 bytes).
	keyLen := uint32(len(want)) //nolint:gosec // bounded by maxKeyLen
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, keyLen)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return VerifyResult{Valid: false}, nil
	}
	def := DefaultParams()
	needsRehash := p.Memory != def.Memory || p.Iterations != def.Iterations ||
		p.Parallelism != def.Parallelism || keyLen != def.KeyLen
	return VerifyResult{Valid: true, NeedsRehash: needsRehash}, nil
}

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("auth: unsupported password hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, errors.New("auth: unsupported argon2 version")
	}

	var p Params
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return Params{}, nil, nil, errors.New("auth: malformed argon2 parameters")
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return Params{}, nil, nil, errors.New("auth: malformed argon2 parameters")
		}
		switch k {
		case "m":
			if n > maxArgonBound {
				return Params{}, nil, nil, errors.New("auth: argon2 memory out of range")
			}
			p.Memory = uint32(n) //nolint:gosec // bounded above
		case "t":
			if n > maxArgonBound {
				return Params{}, nil, nil, errors.New("auth: argon2 iterations out of range")
			}
			p.Iterations = uint32(n) //nolint:gosec // bounded above
		case "p":
			if n > 255 {
				return Params{}, nil, nil, errors.New("auth: argon2 parallelism out of range")
			}
			p.Parallelism = uint8(n)
		}
	}
	if p.Memory == 0 || p.Iterations == 0 || p.Parallelism == 0 {
		return Params{}, nil, nil, errors.New("auth: incomplete argon2 parameters")
	}

	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errors.New("auth: malformed salt")
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errors.New("auth: malformed hash")
	}
	if len(salt) == 0 || len(salt) > maxArgonBound || len(key) == 0 || len(key) > maxArgonBound {
		return Params{}, nil, nil, errors.New("auth: argon2 salt or key out of range")
	}
	p.SaltLen = uint32(len(salt)) //nolint:gosec // bounded above
	p.KeyLen = uint32(len(key))   //nolint:gosec // bounded above
	return p, salt, key, nil
}

// ValidatePassword enforces docs/12-auth.md §2.1: 12-128 characters, not a
// well-known password and not containing the email local part.
//
// A full zxcvbn-style strength estimator is intentionally out of scope for
// E1-S5a; when it is added it belongs here, behind the same signature.
func ValidatePassword(password, email string) error {
	if len(password) < 12 {
		return fmt.Errorf("%w: must be at least 12 characters", ErrWeakPassword)
	}
	if len(password) > 128 {
		return fmt.Errorf("%w: must be at most 128 characters", ErrWeakPassword)
	}
	lower := strings.ToLower(password)
	if commonPasswords[lower] {
		return fmt.Errorf("%w: is too common", ErrWeakPassword)
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 4 {
		if strings.Contains(lower, local) {
			return fmt.Errorf("%w: must not contain the email address", ErrWeakPassword)
		}
	}
	return nil
}

const maxArgonBound = 1 << 20

var commonPasswords = map[string]bool{
	"password1234": true, "123456789012": true, "qwertyuiop12": true,
	"letmein12345": true, "iloveyou1234": true, "admin1234567": true,
	"welcome12345": true, "changeme1234": true, "passw0rdpass": true,
}
