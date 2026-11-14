package agentplugins

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct{ created bool }

func (f *fakeStore) Create(context.Context, string, string, string, string, string, string, []string) (Plugin, error) {
	f.created = true
	return Plugin{ID: "p1"}, nil
}
func (f *fakeStore) List(context.Context, string) ([]Plugin, error)                 { return nil, nil }
func (f *fakeStore) SetProjectPlugin(context.Context, string, string, string) error { return nil }
func (f *fakeStore) ForProject(context.Context, string, string) (Plugin, error) {
	return Plugin{}, domain.ErrNotFound
}
func (f *fakeStore) Audit(context.Context, string, string, string, string, []byte) error { return nil }

func TestCreateEnforcesDigestAllowlist(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, []string{"sha256:good"}, nil)
	_, err := svc.Create(context.Background(), "org", "actor", "plugin", "1", "img@sha256:bad", "run", []string{"development"})
	if !errors.Is(err, domain.ErrValidation) || store.created {
		t.Fatalf("bad digest: err=%v created=%v", err, store.created)
	}
	if _, err := svc.Create(context.Background(), "org", "actor", "plugin", "1", "img@sha256:good", "run", []string{"development"}); err != nil {
		t.Fatalf("good digest: %v", err)
	}
}

type fakeVerifier struct {
	calls int
	err   error
}

func (f *fakeVerifier) Verify(context.Context, string) error { f.calls++; return f.err }

func TestCreateRunsImageVerifier(t *testing.T) {
	verifier := &fakeVerifier{}
	svc := New(&fakeStore{}, nil, verifier)
	if _, err := svc.Create(context.Background(), "org", "actor", "plugin", "1", "img@sha256:good", "run", []string{"development"}); err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 {
		t.Fatalf("calls=%d", verifier.calls)
	}
	bad := &fakeVerifier{err: errors.New("unsigned")}
	svc = New(&fakeStore{}, nil, bad)
	if _, err := svc.Create(context.Background(), "org", "actor", "plugin", "1", "img@sha256:good", "run", []string{"development"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("verifier error: %v", err)
	}
}
