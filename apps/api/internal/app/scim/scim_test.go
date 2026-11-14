package scim

import (
	"context"
	"testing"
)

type fakeStore struct{ org string }

func (f fakeStore) CreateToken(context.Context, string, string, string) (Token, error) {
	return Token{ID: "t", Name: "okta"}, nil
}
func (f fakeStore) ListTokens(context.Context, string) ([]Token, error) { return nil, nil }
func (f fakeStore) RevokeToken(context.Context, string, string) error   { return nil }
func (f fakeStore) TokenOrg(context.Context, string, string) (string, error) {
	return f.org, nil
}
func (f fakeStore) Users(context.Context, string) ([]User, error)   { return nil, nil }
func (f fakeStore) OwnerID(context.Context, string) (string, error) { return "owner", nil }
func (f fakeStore) MembershipRole(context.Context, string, string) (string, error) {
	return "member", nil
}
func (f fakeStore) RemoveMembership(context.Context, string, string) error             { return nil }
func (f fakeStore) UpdateMembershipRole(context.Context, string, string, string) error { return nil }
func (f fakeStore) UpdateUserName(context.Context, string, string, string) error       { return nil }
func (f fakeStore) UpdateUserEmail(context.Context, string, string, string) error      { return nil }

func TestTokenRoundTrip(t *testing.T) {
	svc := New(fakeStore{org: "org-1"}, nil)
	_, raw, err := svc.CreateToken(context.Background(), "org-1", "okta")
	if err != nil {
		t.Fatal(err)
	}
	org, err := svc.Authenticate(context.Background(), raw)
	if err != nil || org != "org-1" {
		t.Fatalf("org=%q err=%v", org, err)
	}
	if _, err := svc.Authenticate(context.Background(), "nope"); err == nil {
		t.Fatal("invalid token must fail")
	}
}
