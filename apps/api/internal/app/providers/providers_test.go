package providers

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct{ stored Stored }

func (f *fakeStore) Create(context.Context, Stored) (Stored, error) { return f.stored, nil }
func (f *fakeStore) List(context.Context, string) ([]Stored, error) { return nil, nil }
func (f *fakeStore) Get(context.Context, string, string) (Stored, error) {
	return f.stored, nil
}
func (f *fakeStore) Default(context.Context, string) (Stored, error) { return f.stored, nil }
func (f *fakeStore) RotateKey(_ context.Context, _ string, _ string, enc []byte, hint string) (Stored, error) {
	f.stored.APIKeyEnc = enc
	f.stored.KeyHint = hint
	return f.stored, nil
}
func (f *fakeStore) SetEnabled(_ context.Context, _ string, _ string, enabled bool) (Stored, error) {
	f.stored.Enabled = enabled
	return f.stored, nil
}
func (f *fakeStore) Delete(context.Context, string, string) error { return nil }

func TestRotateRequiresCipher(t *testing.T) {
	svc, err := New(&fakeStore{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RotateKey(context.Background(), "o", "p", "sk-x"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v", err)
	}
	if _, err := svc.RotateKey(context.Background(), "o", "p", "  "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty key err = %v", err)
	}
}

func TestRotateEncryptsAndKeepsHint(t *testing.T) {
	cipher, err := NewCipher(strings.Repeat("00", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	svc, _ := New(store, cipher, nil)
	provider, err := svc.RotateKey(context.Background(), "o", "p", "sk-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	if provider.KeyHint != "…7890" {
		t.Fatalf("hint = %q", provider.KeyHint)
	}
	if len(store.stored.APIKeyEnc) == 0 {
		t.Fatal("ciphertext not stored")
	}
	plain, err := cipher.Decrypt(store.stored.APIKeyEnc)
	if err != nil || plain != "sk-1234567890" {
		t.Fatalf("decrypt = %q, %v", plain, err)
	}
}

func TestCipherRejectsBadKey(t *testing.T) {
	if _, err := NewCipher("short"); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestEnvelopeIsVersionedAndLegacyDecrypts(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	c, err := NewCipher(hex.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if enc[0] != 2 {
		t.Fatalf("version = %d", enc[0])
	}
	legacy, err := c.seal(c.key, []byte("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.Decrypt(legacy)
	if err != nil || plain != "legacy" {
		t.Fatalf("legacy plain=%q err=%v", plain, err)
	}
}

type fakeKMS struct{}

func (fakeKMS) Wrap(_ context.Context, plain []byte) ([]byte, error) {
	return append([]byte("kms:"), plain...), nil
}

func (fakeKMS) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	return wrapped[len("kms:"):], nil
}

func TestKMSEnvelopeRoundTrip(t *testing.T) {
	c, err := NewCipherWithKMS(strings.Repeat("11", 32), fakeKMS{})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("key")
	if err != nil {
		t.Fatal(err)
	}
	if enc[0] != 3 {
		t.Fatalf("version = %d", enc[0])
	}
	plain, err := c.Decrypt(enc)
	if err != nil || plain != "key" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}

func (fakeKMS) WrapKey(_ context.Context, keyID string, plain []byte) ([]byte, error) {
	return append([]byte("kms:"+keyID+":"), plain...), nil
}

func (fakeKMS) UnwrapKey(_ context.Context, keyID string, wrapped []byte) ([]byte, error) {
	prefix := "kms:" + keyID + ":"
	if len(wrapped) < len(prefix) {
		return nil, errors.New("short")
	}
	return wrapped[len(prefix):], nil
}

func TestBYOKEnvelopeRoundTrip(t *testing.T) {
	c, err := NewCipherWithKMSKey(strings.Repeat("22", 32), fakeKMS{}, "org-key")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt("secret")
	if err != nil {
		t.Fatal(err)
	}
	if enc[0] != 4 {
		t.Fatalf("version=%d", enc[0])
	}
	plain, err := c.Decrypt(enc)
	if err != nil || plain != "secret" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}
