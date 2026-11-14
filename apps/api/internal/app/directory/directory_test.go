package directory

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	members []Member
	updated string
	removed string
	audits  int
}

func (f *fakeStore) Repos(context.Context, string, int) ([]Repo, error) { return nil, nil }
func (f *fakeStore) Runs(context.Context, string, string, string, int) ([]Run, error) {
	return nil, nil
}
func (f *fakeStore) Members(context.Context, string, int) ([]Member, error)   { return f.members, nil }
func (f *fakeStore) Audit(context.Context, string, int) ([]AuditEntry, error) { return nil, nil }
func (f *fakeStore) UpdateRole(_ context.Context, _, userID, role string) error {
	f.updated = userID + ":" + role
	return nil
}
func (f *fakeStore) RemoveMember(_ context.Context, _, userID string) error {
	f.removed = userID
	return nil
}
func (f *fakeStore) RecordAudit(context.Context, string, string, string, string, []byte) error {
	f.audits++
	return nil
}

func TestUpdateRoleProtectsOwnerAndValidates(t *testing.T) {
	store := &fakeStore{members: []Member{
		{UserID: "owner-1", Role: "owner"},
		{UserID: "member-1", Role: "member"},
	}}
	svc := New(store)
	if err := svc.UpdateRole(context.Background(), "o", "actor", "owner-1", "admin"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("owner change err = %v", err)
	}
	if err := svc.UpdateRole(context.Background(), "o", "actor", "member-1", "superuser"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad role err = %v", err)
	}
	if err := svc.UpdateRole(context.Background(), "o", "actor", "member-1", "admin"); err != nil {
		t.Fatal(err)
	}
	if store.updated != "member-1:admin" || store.audits != 1 {
		t.Fatalf("updated=%q audits=%d", store.updated, store.audits)
	}
	if err := svc.UpdateRole(context.Background(), "o", "actor", "missing", "admin"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestRemoveMemberGuards(t *testing.T) {
	store := &fakeStore{members: []Member{
		{UserID: "owner-1", Role: "owner"},
		{UserID: "member-1", Role: "member"},
	}}
	svc := New(store)
	if err := svc.RemoveMember(context.Background(), "o", "actor", "actor"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("self removal err = %v", err)
	}
	if err := svc.RemoveMember(context.Background(), "o", "actor", "owner-1"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("owner removal err = %v", err)
	}
	if err := svc.RemoveMember(context.Background(), "o", "actor", "member-1"); err != nil {
		t.Fatal(err)
	}
	if store.removed != "member-1" {
		t.Fatalf("removed = %q", store.removed)
	}
}
