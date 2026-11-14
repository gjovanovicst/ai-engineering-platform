package designstandards

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	standard Standard
	created  []Standard
	next     int
}

func (f *fakeStore) Create(_ context.Context, orgID, name string, version int, spec, sha []byte) (Standard, error) {
	s := Standard{ID: "ds-" + name, OrgID: orgID, Name: name, Version: version, Status: "draft", Spec: spec}
	f.created = append(f.created, s)
	return s, nil
}
func (f *fakeStore) NextVersion(context.Context, string, string) (int, error) { return f.next, nil }
func (f *fakeStore) List(context.Context, string, int) ([]Standard, error)    { return nil, nil }
func (f *fakeStore) Get(context.Context, string, string) (Standard, error)    { return f.standard, nil }
func (f *fakeStore) Publish(context.Context, string, string) (Standard, error) {
	s := f.standard
	s.Status = "published"
	return s, nil
}
func (f *fakeStore) Audit(context.Context, string, string, string, string, []byte) error { return nil }

func TestValidate(t *testing.T) {
	if _, _, err := Validate([]byte(`{"schema_version":"1","tokens":[]}`)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty tokens err = %v", err)
	}
	if _, _, err := Validate([]byte(`{"schema_version":"1","tokens":[{"name":"--a","value":"#fff","category":"color"},{"name":"--a","value":"#000","category":"color"}]}`)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("duplicate err = %v", err)
	}
	if _, canonical, err := Validate([]byte(`{"schema_version":"1","tokens":[{"name":"--a","value":"#fff","category":"color"}]}`)); err != nil || len(canonical) == 0 {
		t.Fatalf("valid spec err = %v", err)
	}
}

func TestCreateVersionAndPublish(t *testing.T) {
	store := &fakeStore{standard: Standard{ID: "ds-1", Name: "Acme UI", Version: 1, Status: "draft", Spec: []byte(`{"schema_version":"1","tokens":[{"name":"--a","value":"#fff","category":"color"}]}`)}}
	svc := New(store)
	spec := []byte(`{"schema_version":"1","tokens":[{"name":"--brand","value":"#5b5bd6","category":"color"}]}`)
	created, err := svc.Create(context.Background(), "o", "u", "Acme UI", spec)
	if err != nil || created.Version != 1 {
		t.Fatalf("create = %+v err %v", created, err)
	}
	store.next = 2
	version, err := svc.CreateVersion(context.Background(), "o", "u", "ds-1", nil)
	if err != nil || version.Version != 2 {
		t.Fatalf("version = %+v err %v", version, err)
	}
	published, err := svc.Publish(context.Background(), "o", "u", "ds-1")
	if err != nil || published.Status != "published" {
		t.Fatalf("publish = %+v err %v", published, err)
	}
}
