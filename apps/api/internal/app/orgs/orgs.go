// Package orgs manages the tenant record itself.
package orgs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Org is a tenant.
type Org struct {
	ID                string
	Slug              string
	Name              string
	MaxConcurrentRuns int
	LegalHold         bool
	Region            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Store persists organisations.
type Store interface {
	Get(ctx context.Context, id string) (Org, error)
	UpdateName(ctx context.Context, id, name string) (Org, error)
	Delete(ctx context.Context, id string) error
	SetLegalHold(ctx context.Context, id string, hold bool) (Org, error)
	SetRegion(ctx context.Context, id, region string) (Org, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service exposes org use cases.
type Service struct{ store Store }

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

// Get returns the org.
func (s *Service) Get(ctx context.Context, orgID string) (Org, error) {
	return s.store.Get(ctx, orgID)
}

// Rename changes the display name (the slug is immutable).
func (s *Service) Rename(ctx context.Context, orgID, actorID, name string) (Org, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 80 {
		return Org{}, fmt.Errorf("%w: name must be 2-80 characters", domain.ErrValidation)
	}
	org, err := s.store.UpdateName(ctx, orgID, name)
	if err != nil {
		return Org{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "org.renamed", orgID, []byte(`{}`))
	return org, nil
}

// Delete removes the org and everything it owns. The caller must repeat the slug.
func (s *Service) Delete(ctx context.Context, orgID, actorID, confirmSlug string) error {
	org, err := s.store.Get(ctx, orgID)
	if err != nil {
		return err
	}
	if confirmSlug != org.Slug {
		return fmt.Errorf("%w: type the slug %q to confirm", domain.ErrValidation, org.Slug)
	}
	if err := s.store.Delete(ctx, orgID); err != nil {
		return err
	}
	return nil
}

// SetLegalHold toggles the org legal hold (retention must skip its evidence).
func (s *Service) SetLegalHold(ctx context.Context, orgID, actorID string, hold bool) (Org, error) {
	org, err := s.store.SetLegalHold(ctx, orgID, hold)
	if err != nil {
		return Org{}, err
	}
	action := "org.legal_hold.cleared"
	if hold {
		action = "org.legal_hold.set"
	}
	_ = s.store.Audit(ctx, orgID, actorID, action, orgID, []byte(`{}`))
	return org, nil
}

var regions = map[string]bool{"eu-central": true, "eu-west": true, "us-east": true, "us-west": true, "ap-southeast": true}

// SetRegion records the org residency label.
func (s *Service) SetRegion(ctx context.Context, orgID, actorID, region string) (Org, error) {
	region = strings.ToLower(strings.TrimSpace(region))
	if !regions[region] {
		return Org{}, fmt.Errorf("%w: unknown region", domain.ErrValidation)
	}
	org, err := s.store.SetRegion(ctx, orgID, region)
	if err != nil {
		return Org{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "org.region.changed", orgID, []byte(`{"region":"`+region+`"}`))
	return org, nil
}
