package repocreds

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	rec           Record
	have          bool
	upserted      Record
	upsertAction  string
	deleteAction  string
	upsertedActor string
}

func (f *fakeStore) Upsert(_ context.Context, _, repoID, actorID, action string, rec Record) (Record, error) {
	f.upserted = rec
	f.upsertAction = action
	f.upsertedActor = actorID
	f.rec = rec
	f.rec.RepoID = repoID
	f.rec.CreatedAt = time.Now().UTC()
	f.rec.UpdatedAt = f.rec.CreatedAt
	f.have = true
	return f.rec, nil
}

func (f *fakeStore) Get(context.Context, string, string) (Record, error) {
	if !f.have {
		return Record{}, domain.ErrNotFound
	}
	return f.rec, nil
}

func (f *fakeStore) Delete(_ context.Context, _, _, _, action string) error {
	f.deleteAction = action
	f.have = false
	return nil
}

// fakeCipher marks ciphertext so a test can prove the plaintext is not stored.
type fakeCipher struct{}

func (fakeCipher) Encrypt(plain string) ([]byte, error) { return []byte("sealed:" + plain), nil }
func (fakeCipher) Decrypt(data []byte) (string, error) {
	s := string(data)
	if !strings.HasPrefix(s, "sealed:") {
		return "", errors.New("bad ciphertext")
	}
	return strings.TrimPrefix(s, "sealed:"), nil
}

func newService(t *testing.T, cipher Cipher) (*Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{}
	svc, err := New(store, cipher)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc, store
}

func TestSetRejectsInvalidTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"too short", "abc123"},
		{"whitespace", "github_pat_ very secret"},
		{"too long", strings.Repeat("a", maxTokenLen+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, store := newService(t, fakeCipher{})
			if _, err := svc.Set(context.Background(), "org-1", "repo-1", "user-1", tc.token); !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("err = %v, want ErrValidation", err)
			}
			if store.upserted.Ciphertext != nil {
				t.Fatal("an invalid token must not reach the store")
			}
		})
	}
}

func TestSetRequiresASecretsKey(t *testing.T) {
	t.Parallel()
	svc, store := newService(t, nil)
	_, err := svc.Set(context.Background(), "org-1", "repo-1", "user-1", "github_pat_1234567890")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
	if !strings.Contains(err.Error(), "AIP_SECRETS_KEY") {
		t.Fatalf("err = %v, want a hint about AIP_SECRETS_KEY", err)
	}
	if store.upserted.Ciphertext != nil {
		t.Fatal("a plaintext token must never be stored")
	}
}

func TestSetStoresSealedTokenAndHint(t *testing.T) {
	t.Parallel()
	svc, store := newService(t, fakeCipher{})
	st, err := svc.Set(context.Background(), "org-1", "repo-1", "user-1", "github_pat_abcdef123456")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !st.Configured || st.Hint != "…3456" {
		t.Fatalf("status = %+v, want configured with hint …3456", st)
	}
	if got := string(store.upserted.Ciphertext); got != "sealed:github_pat_abcdef123456" {
		t.Fatalf("ciphertext = %q, want the sealed token", got)
	}
	if store.upserted.Hint != "…3456" {
		t.Fatalf("stored hint = %q", store.upserted.Hint)
	}
	if store.upsertAction != "repo.credential.set" {
		t.Fatalf("audit action = %q", store.upsertAction)
	}
	if store.upsertedActor != "user-1" {
		t.Fatalf("actor = %q", store.upsertedActor)
	}
}

func TestTokenDecryptsAndFallsBackWhenAbsent(t *testing.T) {
	t.Parallel()
	svc, _ := newService(t, fakeCipher{})

	token, err := svc.Token(context.Background(), "org-1", "repo-1")
	if err != nil || token != "" {
		t.Fatalf("Token on empty store = %q, %v; want empty and no error", token, err)
	}

	if _, err := svc.Set(context.Background(), "org-1", "repo-1", "user-1", "glpat-1234567890"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	token, err = svc.Token(context.Background(), "org-1", "repo-1")
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "glpat-1234567890" {
		t.Fatalf("token = %q", token)
	}
}

func TestStatusIsEmptyWithoutCredential(t *testing.T) {
	t.Parallel()
	svc, _ := newService(t, fakeCipher{})
	st, err := svc.Status(context.Background(), "org-1", "repo-1")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Configured || st.UpdatedAt != nil {
		t.Fatalf("status = %+v, want not configured", st)
	}
}

func TestDeleteAuditsTheRemoval(t *testing.T) {
	t.Parallel()
	svc, store := newService(t, fakeCipher{})
	if err := svc.Delete(context.Background(), "org-1", "repo-1", "user-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.deleteAction != "repo.credential.deleted" {
		t.Fatalf("audit action = %q", store.deleteAction)
	}
}

func TestHintNeverLeaksMoreThanFourCharacters(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                    "…",
		"abc":                 "…",
		"github_pat_abcdef1c": "…ef1c",
	}
	for token, want := range cases {
		if got := Hint(token); got != want {
			t.Fatalf("Hint(%q) = %q, want %q", token, got, want)
		}
	}
}
