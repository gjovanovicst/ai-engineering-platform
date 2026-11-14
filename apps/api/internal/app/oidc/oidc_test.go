package oidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

type fakeAccounts struct {
	identity auth.OIDCIdentity
}

func (f *fakeAccounts) OIDCLogin(_ context.Context, id auth.OIDCIdentity, _ auth.Platform) (auth.TokenPair, error) {
	f.identity = id
	return auth.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func providerFor(t *testing.T, server *httptest.Server) Provider {
	t.Helper()
	return Provider{
		Name: "fake", ClientID: "id", ClientSecret: "secret",
		AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token",
		UserInfoURL: server.URL + "/user", RedirectURL: "http://localhost/cb",
		Scopes: []string{"openid", "email"},
	}
}

func TestCallbackExchangesCode(t *testing.T) {
	accounts := &fakeAccounts{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil || r.Form.Get("code") != "code-1" {
				t.Errorf("token form = %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-1"}`))
		case "/user":
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"sub":"u-1","email":"a@b.co","name":"A"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	svc, err := New([]Provider{providerFor(t, server)}, accounts)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := svc.Callback(context.Background(), "fake", "code-1")
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessToken != "access" {
		t.Fatalf("pair = %+v", pair)
	}
	if accounts.identity.Subject != "u-1" || accounts.identity.Email != "a@b.co" || accounts.identity.Provider != "fake" {
		t.Fatalf("identity = %+v", accounts.identity)
	}
}

func TestStartBuildsAuthorizeURL(t *testing.T) {
	svc, _ := New([]Provider{{
		Name: "fake", ClientID: "id", ClientSecret: "secret",
		AuthorizeURL: "https://example.com/authorize", TokenURL: "https://example.com/token",
		UserInfoURL: "https://example.com/userinfo", RedirectURL: "http://localhost/cb",
		Scopes: []string{"openid", "email"},
	}}, &fakeAccounts{})
	target, state, err := svc.Start("fake")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"client_id=id", "response_type=code", "state=" + state, "scope=openid+email"} {
		if !strings.Contains(target, want) {
			t.Fatalf("url %q missing %q", target, want)
		}
	}
}

func TestUnconfiguredProviderIsDisabled(t *testing.T) {
	svc, err := New([]Provider{{Name: "github"}}, &fakeAccounts{})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled("github") {
		t.Fatal("provider without credentials must be disabled")
	}
	if _, _, err := svc.Start("github"); !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("err = %v", err)
	}
}

func TestCallbackRejectsFailedExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	svc, _ := New([]Provider{providerFor(t, server)}, &fakeAccounts{})
	if _, err := svc.Callback(context.Background(), "fake", "bad"); err == nil {
		t.Fatal("expected an exchange error")
	}
}
