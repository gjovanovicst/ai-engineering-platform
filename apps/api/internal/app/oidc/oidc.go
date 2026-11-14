// Package oidc implements the OIDC authorization-code flow for GitHub and Google.
package oidc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// Provider is one configured OIDC provider.
type Provider struct {
	Name         string
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	EmailsURL    string // GitHub only
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// Accounts signs in the verified identity.
type Accounts interface {
	OIDCLogin(ctx context.Context, id auth.OIDCIdentity, platform auth.Platform) (auth.TokenPair, error)
}

// Service runs the authorization-code flow.
type Service struct {
	providers map[string]Provider
	accounts  Accounts
	client    *http.Client
}

// New builds the service from configured providers.
func New(providers []Provider, accounts Accounts) (*Service, error) {
	if accounts == nil {
		return nil, errors.New("oidc: accounts is required")
	}
	reg := map[string]Provider{}
	for _, p := range providers {
		if p.ClientID == "" || p.ClientSecret == "" || p.AuthorizeURL == "" || p.TokenURL == "" {
			continue // provider not configured
		}
		reg[p.Name] = p
	}
	return &Service{providers: reg, accounts: accounts, client: http.DefaultClient}, nil
}

// Enabled reports whether a provider is configured.
func (s *Service) Enabled(name string) bool {
	_, ok := s.providers[name]
	return ok
}

// Start returns the authorize URL and the CSRF state.
func (s *Service) Start(name string) (string, string, error) {
	p, ok := s.providers[name]
	if !ok {
		return "", "", ErrProviderDisabled
	}
	state, err := randomState()
	if err != nil {
		return "", "", err
	}
	q := url.Values{
		"client_id":     {p.ClientID},
		"redirect_uri":  {p.RedirectURL},
		"response_type": {"code"},
		"scope":         {strings.Join(p.Scopes, " ")},
		"state":         {state},
	}
	return p.AuthorizeURL + "?" + q.Encode(), state, nil
}

// ErrProviderDisabled means the provider has no client credentials.
var ErrProviderDisabled = errors.New("oidc: provider is not configured")

// ErrExchange means the provider rejected the code or returned no identity.
var ErrExchange = errors.New("oidc: exchange failed")

// Callback exchanges the code and signs the user in.
func (s *Service) Callback(ctx context.Context, name, code string) (auth.TokenPair, error) {
	p, ok := s.providers[name]
	if !ok {
		return auth.TokenPair{}, ErrProviderDisabled
	}
	token, err := s.exchange(ctx, p, code)
	if err != nil {
		return auth.TokenPair{}, err
	}
	identity, err := s.userInfo(ctx, p, token)
	if err != nil {
		return auth.TokenPair{}, err
	}
	return s.accounts.OIDCLogin(ctx, identity, auth.PlatformWeb)
}

func (s *Service) exchange(ctx context.Context, p Provider, code string) (string, error) {
	form := url.Values{
		"client_id":     {p.ClientID},
		"client_secret": {p.ClientSecret},
		"code":          {code},
		"redirect_uri":  {p.RedirectURL},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrExchange, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("%w: status %d", ErrExchange, res.StatusCode)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return "", fmt.Errorf("%w: no access token", ErrExchange)
	}
	return payload.AccessToken, nil
}

func (s *Service) userInfo(ctx context.Context, p Provider, token string) (auth.OIDCIdentity, error) {
	get := func(target string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		res, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if res.StatusCode >= 300 {
			return nil, fmt.Errorf("%w: userinfo status %d", ErrExchange, res.StatusCode)
		}
		return body, nil
	}

	body, err := get(p.UserInfoURL)
	if err != nil {
		return auth.OIDCIdentity{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return auth.OIDCIdentity{}, fmt.Errorf("%w: userinfo decode", ErrExchange)
	}

	subject := stringField(raw, "sub")
	if subject == "" {
		subject = stringField(raw, "id") // GitHub numeric id
	}
	email := stringField(raw, "email")
	name := stringField(raw, "name")
	if name == "" {
		name = stringField(raw, "login")
	}
	if subject == "" || email == "" {
		if p.EmailsURL != "" {
			if emails, err := get(p.EmailsURL); err == nil {
				var list []struct {
					Email    string `json:"email"`
					Primary  bool   `json:"primary"`
					Verified bool   `json:"verified"`
				}
				if json.Unmarshal(emails, &list) == nil {
					for _, candidate := range list {
						if candidate.Verified && email == "" {
							email = candidate.Email
						}
					}
				}
			}
		}
	}
	if subject == "" || email == "" {
		return auth.OIDCIdentity{}, fmt.Errorf("%w: identity is missing a subject or email", ErrExchange)
	}
	return auth.OIDCIdentity{Provider: p.Name, Subject: subject, Email: email, Name: name}, nil
}

func stringField(raw map[string]any, key string) string {
	switch value := raw[key].(type) {
	case string:
		return value
	case float64:
		return fmt.Sprintf("%.0f", value)
	default:
		return ""
	}
}

func randomState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
