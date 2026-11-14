package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Discovery is the subset of an OIDC discovery document we need.
type Discovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

// Discover fetches {issuer}/.well-known/openid-configuration.
func Discover(ctx context.Context, issuer string) (Discovery, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return Discovery{}, fmt.Errorf("oidc: issuer is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return Discovery{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Discovery{}, fmt.Errorf("oidc: discovery: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return Discovery{}, fmt.Errorf("oidc: discovery status %d", res.StatusCode)
	}
	var doc Discovery
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return Discovery{}, fmt.Errorf("oidc: discovery decode: %w", err)
	}
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return Discovery{}, fmt.Errorf("oidc: discovery is missing endpoints")
	}
	return doc, nil
}
