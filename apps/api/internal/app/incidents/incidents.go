// Package incidents records production incidents for DORA change-failure rate.
package incidents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Incident is a production incident.
type Incident struct {
	ID         string
	ProjectID  *string
	Title      string
	Severity   string
	Status     string
	OpenedAt   time.Time
	ResolvedAt *time.Time
}

// Store persists incidents.
type Store interface {
	Create(ctx context.Context, orgID, projectID, title, severity, userID string) (Incident, error)
	List(ctx context.Context, orgID string, limit int) ([]Incident, error)
	Resolve(ctx context.Context, orgID, id string) (Incident, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service exposes incident use cases.
type Service struct{ store Store }

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

var severities = map[string]bool{"sev1": true, "sev2": true, "sev3": true, "sev4": true}

// Create validates and records an incident.
func (s *Service) Create(ctx context.Context, orgID, projectID, title, severity, userID string) (Incident, error) {
	title = strings.TrimSpace(title)
	severity = strings.ToLower(strings.TrimSpace(severity))
	if len(title) < 3 || len(title) > 200 {
		return Incident{}, fmt.Errorf("%w: title must be 3..200 characters", domain.ErrValidation)
	}
	if !severities[severity] {
		return Incident{}, fmt.Errorf("%w: severity must be sev1..sev4", domain.ErrValidation)
	}
	incident, err := s.store.Create(ctx, orgID, projectID, title, severity, userID)
	if err != nil {
		return Incident{}, err
	}
	_ = s.store.Audit(ctx, orgID, userID, "incident.opened", incident.ID, []byte(`{}`))
	return incident, nil
}

// List returns recent incidents.
func (s *Service) List(ctx context.Context, orgID string, limit int) ([]Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.List(ctx, orgID, limit)
}

// Resolve closes an open incident.
func (s *Service) Resolve(ctx context.Context, orgID, id, userID string) (Incident, error) {
	incident, err := s.store.Resolve(ctx, orgID, id)
	if err != nil {
		return Incident{}, err
	}
	_ = s.store.Audit(ctx, orgID, userID, "incident.resolved", incident.ID, []byte(`{}`))
	return incident, nil
}
