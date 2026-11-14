// Package githubsetup implements the GitHub App manifest onboarding flow.
package githubsetup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	githubapp "github.com/gjovanovicst/ai-engineering-platform/internal/platform/githubapp"
)

// Cipher encrypts app secrets.
type Cipher interface {
	Encrypt(string) ([]byte, error)
	Decrypt([]byte) (string, error)
}

// Store persists the app and its one-time states.
type Store interface {
	Upsert(ctx context.Context, orgID string, in Record) error
	Get(ctx context.Context, orgID string) (Record, error)
	SetInstallation(ctx context.Context, orgID string, installationID int64) error
	CreateState(ctx context.Context, state, orgID string) error
	ConsumeState(ctx context.Context, state string) (string, error)
}

// Record is the stored app configuration.
type Record struct {
	AppID          int64
	Slug           string
	PEM            string
	WebhookSecret  string
	InstallURL     string
	InstallationID int64
}

// Status is the API-facing view.
type Status struct {
	AppID          int64  `json:"app_id"`
	Slug           string `json:"slug"`
	InstallURL     string `json:"install_url"`
	InstallationID int64  `json:"installation_id"`
	Installed      bool   `json:"installed"`
}

// ManifestResult is what the UI posts to GitHub.
type ManifestResult struct {
	ActionURL string          `json:"action_url"`
	State     string          `json:"state"`
	Manifest  json.RawMessage `json:"manifest"`
}

// Service creates apps and exchanges the manifest code.
type Service struct {
	store   Store
	cipher  Cipher
	apiURL  string
	webURL  string
	http    *http.Client
	baseAPI string
}

// New wires the service.
func New(store Store, cipher Cipher, apiURL, webURL, baseAPI string) *Service {
	if baseAPI == "" {
		baseAPI = "https://api.github.com"
	}
	return &Service{store: store, cipher: cipher, apiURL: apiURL, webURL: webURL, http: http.DefaultClient, baseAPI: strings.TrimRight(baseAPI, "/")}
}

// CreateManifest stores a state and returns the manifest for GitHub.
func (s *Service) CreateManifest(ctx context.Context, orgID string) (ManifestResult, error) {
	if s.cipher == nil {
		return ManifestResult{}, fmt.Errorf("%w: AIP_SECRETS_KEY is required to store GitHub App credentials", domain.ErrValidation)
	}
	state, err := randomState()
	if err != nil {
		return ManifestResult{}, err
	}
	if err := s.store.CreateState(ctx, state, orgID); err != nil {
		return ManifestResult{}, err
	}
	setupState, err := randomState()
	if err != nil {
		return ManifestResult{}, err
	}
	if err := s.store.CreateState(ctx, setupState, orgID); err != nil {
		return ManifestResult{}, err
	}
	hookURL := strings.TrimRight(s.apiURL, "/") + "/webhooks/github-app/" + orgID
	redirect := strings.TrimRight(s.apiURL, "/") + "/api/v1/github-app/callback"
	manifest := map[string]any{
		"name":            "AI Engineering Platform " + orgID[:8],
		"url":             s.webURL,
		"hook_attributes": map[string]any{"url": hookURL, "active": true},
		"redirect_url":    redirect,
		"setup_url":       strings.TrimRight(s.apiURL, "/") + "/api/v1/github-app/setup?state=" + setupState,
		"public":          false,
		"default_permissions": map[string]string{
			"contents": "write", "pull_requests": "write", "checks": "read", "metadata": "read",
		},
		"default_events": []string{"check_run", "pull_request", "pull_request_review"},
	}
	raw, _ := json.Marshal(manifest)
	return ManifestResult{ActionURL: "https://github.com/settings/apps/new", State: state, Manifest: raw}, nil
}

// Complete exchanges the manifest code and stores the credentials.
func (s *Service) Complete(ctx context.Context, state, code string) (Status, error) {
	orgID, err := s.store.ConsumeState(ctx, state)
	if err != nil {
		return Status{}, domain.ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseAPI+"/app-manifests/"+code+"/conversions", nil)
	if err != nil {
		return Status{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := s.http.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return Status{}, fmt.Errorf("github manifest exchange: status %d", res.StatusCode)
	}
	var payload struct {
		ID            int64  `json:"id"`
		Slug          string `json:"slug"`
		PEM           string `json:"pem"`
		WebhookSecret string `json:"webhook_secret"`
		HTMLURL       string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Status{}, err
	}
	pem, err := s.cipher.Encrypt(payload.PEM)
	if err != nil {
		return Status{}, err
	}
	secret, err := s.cipher.Encrypt(payload.WebhookSecret)
	if err != nil {
		return Status{}, err
	}
	record := Record{
		AppID: payload.ID, Slug: payload.Slug,
		PEM: base64.StdEncoding.EncodeToString(pem), WebhookSecret: base64.StdEncoding.EncodeToString(secret),
		InstallURL: payload.HTMLURL + "/installations/new",
	}
	if err := s.store.Upsert(ctx, orgID, record); err != nil {
		return Status{}, err
	}
	return Status{AppID: record.AppID, Slug: record.Slug, InstallURL: record.InstallURL}, nil
}

// InstallationToken mints a short-lived token for the org's installation.
func (s *Service) InstallationToken(ctx context.Context, orgID string) (string, error) {
	record, err := s.store.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	if record.InstallationID == 0 {
		return "", domain.ErrNotFound
	}
	pemRaw, err := base64.StdEncoding.DecodeString(record.PEM)
	if err != nil {
		return "", err
	}
	pemPlain, err := s.cipher.Decrypt(pemRaw)
	if err != nil {
		return "", err
	}
	client, err := githubapp.NewFromPEM(record.AppID, record.InstallationID, []byte(pemPlain), s.baseAPI)
	if err != nil {
		return "", err
	}
	return client.Token(ctx)
}

// WebhookSecret returns the decrypted webhook secret for an org.
func (s *Service) WebhookSecret(ctx context.Context, orgID string) (string, error) {
	record, err := s.store.Get(ctx, orgID)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(record.WebhookSecret)
	if err != nil {
		return "", err
	}
	secret, err := s.cipher.Decrypt(raw)
	if err != nil {
		return "", err
	}
	return secret, nil
}

// Setup consumes the installation state GitHub sends to setup_url.
func (s *Service) Setup(ctx context.Context, state string, installationID int64) (Status, error) {
	orgID, err := s.store.ConsumeState(ctx, state)
	if err != nil {
		return Status{}, domain.ErrNotFound
	}
	return s.SetInstallation(ctx, orgID, installationID)
}

// Status returns the app status for an org.
func (s *Service) Status(ctx context.Context, orgID string) (Status, error) {
	record, err := s.store.Get(ctx, orgID)
	if err != nil {
		return Status{}, err
	}
	return Status{AppID: record.AppID, Slug: record.Slug, InstallURL: record.InstallURL,
		InstallationID: record.InstallationID, Installed: record.InstallationID > 0}, nil
}

// SetInstallation records the installation id.
func (s *Service) SetInstallation(ctx context.Context, orgID string, installationID int64) (Status, error) {
	if installationID <= 0 {
		return Status{}, fmt.Errorf("%w: installation id is required", domain.ErrValidation)
	}
	if err := s.store.SetInstallation(ctx, orgID, installationID); err != nil {
		return Status{}, err
	}
	return s.Status(ctx, orgID)
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// CallbackPage renders a minimal success page.
func CallbackPage(status Status) string {
	return fmt.Sprintf(`<!doctype html><html><body style="font-family:system-ui;padding:40px">
<h1>GitHub App created</h1><p>App <strong>%s</strong> (id %d) is connected.</p>
<p><a href="%s">Install it on your repositories</a>, then paste the installation id into Settings → GitHub App.</p>
</body></html>`, html.EscapeString(status.Slug), status.AppID, html.EscapeString(status.InstallURL))
}

var _ = bytes.NewReader
var _ = time.Now
