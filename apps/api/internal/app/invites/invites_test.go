package invites

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	invite   Invite
	orgID    string
	email    string
	accepted bool
	members  int
}

func (f *fakeStore) Create(_ context.Context, orgID, email, role string, _ []byte, expires time.Time, _ string) (Invite, error) {
	return Invite{ID: "inv-1", OrgID: orgID, Email: email, Role: role, ExpiresAt: expires}, nil
}
func (f *fakeStore) List(context.Context, string, int) ([]Invite, error) { return nil, nil }
func (f *fakeStore) OrgForToken(context.Context, []byte) (string, error) { return f.orgID, nil }
func (f *fakeStore) ByToken(context.Context, string, []byte) (Invite, error) {
	return f.invite, nil
}
func (f *fakeStore) Accept(context.Context, string, string) (Invite, error) {
	f.accepted = true
	return f.invite, nil
}
func (f *fakeStore) AddMembership(context.Context, string, string, string) error {
	f.members++
	return nil
}
func (f *fakeStore) EmailForUser(context.Context, string) (string, error) { return f.email, nil }

func newTestService(t *testing.T, f *fakeStore) *Service {
	t.Helper()
	svc, err := New(f, nil, nil, "http://localhost:3000", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	return svc
}

type fakeAccounts struct{ called bool }

func (f *fakeAccounts) CreateInvitedUser(_ context.Context, email, name, password, orgID, role string) (auth.TokenPair, error) {
	f.called = true
	return auth.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func TestRegisterAcceptCreatesAccount(t *testing.T) {
	store := &fakeStore{orgID: "org-1",
		invite: Invite{ID: "inv-1", OrgID: "org-1", Email: "new@acme.io", Role: "member",
			ExpiresAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}
	accounts := &fakeAccounts{}
	svc, err := New(store, nil, accounts, "http://localhost:3000", nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	pair, err := svc.RegisterAccept(context.Background(), "token", "New User", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessToken != "access" || !accounts.called || !store.accepted {
		t.Fatalf("pair=%+v called=%v accepted=%v", pair, accounts.called, store.accepted)
	}
}

func TestAcceptAddsMembership(t *testing.T) {
	store := &fakeStore{
		orgID: "org-1",
		email: "b@acme.io",
		invite: Invite{ID: "inv-1", OrgID: "org-1", Email: "B@acme.io", Role: "member",
			ExpiresAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
	}
	svc := newTestService(t, store)
	if _, err := svc.Accept(context.Background(), "org-1", "token", "user-2"); err != nil {
		t.Fatal(err)
	}
	if store.members != 1 || !store.accepted {
		t.Fatalf("members=%d accepted=%v", store.members, store.accepted)
	}
}

func TestAcceptRejectsOtherEmail(t *testing.T) {
	store := &fakeStore{orgID: "org-1", email: "someone@acme.io",
		invite: Invite{Email: "invited@acme.io", ExpiresAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)}}
	svc := newTestService(t, store)
	if _, err := svc.Accept(context.Background(), "org-1", "token", "user-2"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v", err)
	}
	if store.members != 0 {
		t.Fatal("membership must not be added")
	}
}

func TestAcceptRejectsExpired(t *testing.T) {
	store := &fakeStore{orgID: "org-1", email: "b@acme.io",
		invite: Invite{Email: "b@acme.io", ExpiresAt: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)}}
	svc := newTestService(t, store)
	if _, err := svc.Accept(context.Background(), "org-1", "token", "user-2"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v", err)
	}
}

func TestAcceptRejectsWrongOrg(t *testing.T) {
	store := &fakeStore{orgID: "org-2", email: "b@acme.io", invite: Invite{Email: "b@acme.io"}}
	svc := newTestService(t, store)
	if _, err := svc.Accept(context.Background(), "org-1", "token", "user-2"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAcceptRejectsUsedInvite(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{orgID: "org-1", email: "b@acme.io",
		invite: Invite{Email: "b@acme.io", ExpiresAt: now.Add(time.Hour), AcceptedAt: &now}}
	svc := newTestService(t, store)
	if _, err := svc.Accept(context.Background(), "org-1", "token", "user-2"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateValidatesEmailAndRole(t *testing.T) {
	svc := newTestService(t, &fakeStore{})
	if _, err := svc.Create(context.Background(), CreateInput{OrgID: "o", Email: "nope", Role: "member"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("email: %v", err)
	}
	if _, err := svc.Create(context.Background(), CreateInput{OrgID: "o", Email: "a@b.co", Role: "root"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("role: %v", err)
	}
}
