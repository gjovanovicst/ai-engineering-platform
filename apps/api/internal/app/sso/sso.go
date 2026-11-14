// Package sso stores per-org SAML configuration metadata.
package sso

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/samlmeta"
)

// Config is the IdP metadata an admin registers.
type Config struct {
	EntityID       string
	MetadataURL    string
	SSOURL         string
	CertificatePEM string
	Enabled        bool
	UpdatedAt      time.Time
}

// Store persists SSO config.
type Store interface {
	Get(ctx context.Context, orgID string) (Config, error)
	Upsert(ctx context.Context, orgID string, cfg Config) (Config, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service exposes SSO config use cases.
type Service struct {
	store        Store
	allowPrivate bool
}

// New wires the service; allowPrivate permits internal IdP metadata URLs.
func New(store Store, allowPrivate bool) *Service {
	return &Service{store: store, allowPrivate: allowPrivate}
}

// Get returns the config or ErrNotFound.
func (s *Service) Get(ctx context.Context, orgID string) (Config, error) {
	return s.store.Get(ctx, orgID)
}

// Set validates and stores the config.
func (s *Service) Set(ctx context.Context, orgID, actorID string, cfg Config) (Config, error) {
	cfg.EntityID = strings.TrimSpace(cfg.EntityID)
	cfg.MetadataURL = strings.TrimSpace(cfg.MetadataURL)
	cfg.SSOURL = strings.TrimSpace(cfg.SSOURL)
	cfg.CertificatePEM = strings.TrimSpace(cfg.CertificatePEM)
	if cfg.EntityID == "" || len(cfg.EntityID) > 500 {
		return Config{}, fmt.Errorf("%w: entity_id is required (max 500)", domain.ErrValidation)
	}
	parsed, err := url.Parse(cfg.MetadataURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return Config{}, fmt.Errorf("%w: metadata_url must be an http(s) URL", domain.ErrValidation)
	}
	if cfg.SSOURL != "" {
		parsed, err := url.Parse(cfg.SSOURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return Config{}, fmt.Errorf("%w: sso_url must be an http(s) URL", domain.ErrValidation)
		}
	}
	if len(cfg.CertificatePEM) > 20000 {
		return Config{}, fmt.Errorf("%w: certificate is too large", domain.ErrValidation)
	}
	out, err := s.store.Upsert(ctx, orgID, cfg)
	if err != nil {
		return Config{}, err
	}
	action := "org.sso.updated"
	if cfg.Enabled {
		action = "org.sso.enabled"
	}
	_ = s.store.Audit(ctx, orgID, actorID, action, orgID, []byte(`{}`))
	return out, nil
}

// Fetch downloads IdP metadata and stores its entity id/certificate. It never
// enables SSO; login remains unimplemented.
func (s *Service) Fetch(ctx context.Context, orgID, actorID, metadataURL string) (Config, error) {
	current, err := s.store.Get(ctx, orgID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return Config{}, err
	}
	if strings.TrimSpace(metadataURL) == "" {
		metadataURL = current.MetadataURL
	}
	if strings.TrimSpace(metadataURL) == "" {
		return Config{}, fmt.Errorf("%w: metadata_url is required", domain.ErrValidation)
	}
	meta, err := samlmeta.Fetch(ctx, strings.TrimSpace(metadataURL), s.allowPrivate)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", domain.ErrValidation, err)
	}
	cfg, err := s.store.Upsert(ctx, orgID, Config{
		EntityID: meta.EntityID, MetadataURL: strings.TrimSpace(metadataURL),
		SSOURL: meta.SSOURL, CertificatePEM: meta.Certificate, Enabled: current.Enabled,
	})
	if err != nil {
		return Config{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "org.sso.metadata_fetched", orgID, []byte(`{}`))
	return cfg, nil
}
