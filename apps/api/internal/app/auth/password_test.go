package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery", DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("hash = %q, want PHC argon2id", hash)
	}

	res, err := VerifyPassword("correct horse battery", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.NeedsRehash {
		t.Fatalf("result = %+v, want valid without rehash", res)
	}

	res, err = VerifyPassword("wrong password", hash)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Fatal("wrong password verified")
	}
}

func TestVerifyNeedsRehash(t *testing.T) {
	weak := Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}
	hash, err := HashPassword("correct horse battery", weak)
	if err != nil {
		t.Fatal(err)
	}
	res, err := VerifyPassword("correct horse battery", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || !res.NeedsRehash {
		t.Fatalf("result = %+v, want valid + needs rehash", res)
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		email    string
		wantErr  bool
	}{
		{"valid", "correct horse battery", "goran@acme.io", false},
		{"too short", "short", "goran@acme.io", true},
		{"common", "password1234", "goran@acme.io", true},
		{"contains email local part", "goran-is-the-best", "goran@acme.io", true},
		{"long but fine", strings.Repeat("a", 128), "goran@acme.io", false},
		{"too long", strings.Repeat("a", 129), "goran@acme.io", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password, tc.email)
			if tc.wantErr {
				if !errors.Is(err, ErrWeakPassword) {
					t.Fatalf("err = %v, want ErrWeakPassword", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
