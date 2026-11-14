package apikeys

import (
	"context"
	"testing"
	"time"
)

func TestParseToken(t *testing.T) {
	org := "11111111-1111-1111-1111-111111111111"
	token := "aip." + "MTExMTExMTEtMTExMS0xMTExLTExMTEtMTExMTExMTExMTEx" + ".secret"
	gotOrg, secret, ok := ParseToken(token)
	if !ok || secret != "secret" || gotOrg != org {
		t.Fatalf("org=%q secret=%q ok=%v", gotOrg, secret, ok)
	}
	if _, _, ok := ParseToken("aip.bad"); ok {
		t.Fatal("short token must fail")
	}
}

type fakeStore struct{ key Key }

func (f *fakeStore) Create(context.Context, string, string, string, string, []string, *time.Time) (Key, error) {
	return f.key, nil
}
func (f *fakeStore) ByHash(context.Context, string, string) (Key, error) { return f.key, nil }
func (f *fakeStore) List(context.Context, string) ([]Key, error)         { return nil, nil }
func (f *fakeStore) Revoke(context.Context, string, string) error        { return nil }
func (f *fakeStore) Touch(context.Context, string) error                 { return nil }

func TestExpiredKeyFailsAuthentication(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	store := &fakeStore{key: Key{ID: "k", OrgID: "o", Role: "admin", ExpiresAt: &past}}
	svc, _ := New(store)
	if _, err := svc.Authenticate(context.Background(), "aip.bvcmc.hashed"); err == nil {
		t.Fatal("expired key must fail")
	}
}
