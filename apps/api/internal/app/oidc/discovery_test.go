package oidc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"authorization_endpoint":"https://idp/auth","token_endpoint":"https://idp/token","userinfo_endpoint":"https://idp/userinfo"}`))
	}))
	defer srv.Close()
	doc, err := Discover(context.Background(), srv.URL+"/")
	if err != nil || doc.AuthorizationEndpoint != "https://idp/auth" || doc.TokenEndpoint != "https://idp/token" {
		t.Fatalf("doc=%+v err=%v", doc, err)
	}
}
