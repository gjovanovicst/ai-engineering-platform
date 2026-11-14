// Package designstandards versions the organisation design standard.
package designstandards

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// ErrNotFound is returned when a standard does not exist.
var ErrNotFound = domain.ErrNotFound

// Standard is one immutable published version (or a draft).
type Standard struct {
	ID          string
	OrgID       string
	Name        string
	Version     int
	Status      string
	Spec        json.RawMessage
	SHA256      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Store persists design standards.
type Store interface {
	Create(ctx context.Context, orgID, name string, version int, spec []byte, sha []byte) (Standard, error)
	NextVersion(ctx context.Context, orgID, name string) (int, error)
	List(ctx context.Context, orgID string, limit int) ([]Standard, error)
	Get(ctx context.Context, orgID, id string) (Standard, error)
	Publish(ctx context.Context, orgID, id string) (Standard, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service validates and versions standards.
type Service struct{ store Store }

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

// Spec is the v1 design standard document.
type Spec struct {
	SchemaVersion string   `json:"schema_version"`
	Tokens        []Token  `json:"tokens"`
	Typography    any      `json:"typography,omitempty"`
	Spacing       any      `json:"spacing,omitempty"`
	Radius        any      `json:"radius,omitempty"`
	Themes        any      `json:"themes,omitempty"`
	Accessibility []string `json:"accessibility,omitempty"`
	Responsive    []string `json:"responsive,omitempty"`
	Patterns      []string `json:"patterns,omitempty"`
}

// Token is one design token.
type Token struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Category string `json:"category"`
}

var tokenCategories = map[string]bool{"color": true, "typography": true, "spacing": true, "radius": true, "layout": true, "shadow": true, "other": true}

// Validate checks the document and returns a canonical encoding.
func Validate(raw []byte) (Spec, []byte, error) {
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return Spec{}, nil, fmt.Errorf("%w: spec is not valid JSON", domain.ErrValidation)
	}
	if spec.SchemaVersion != "1" {
		return Spec{}, nil, fmt.Errorf("%w: schema_version must be \"1\"", domain.ErrValidation)
	}
	if len(spec.Tokens) == 0 {
		return Spec{}, nil, fmt.Errorf("%w: at least one token is required", domain.ErrValidation)
	}
	seen := map[string]bool{}
	for _, token := range spec.Tokens {
		if strings.TrimSpace(token.Name) == "" || strings.TrimSpace(token.Value) == "" {
			return Spec{}, nil, fmt.Errorf("%w: each token needs a name and a value", domain.ErrValidation)
		}
		if seen[token.Name] {
			return Spec{}, nil, fmt.Errorf("%w: duplicate token %s", domain.ErrValidation, token.Name)
		}
		seen[token.Name] = true
		if token.Category == "" {
			token.Category = "other"
		}
		if !tokenCategories[token.Category] {
			return Spec{}, nil, fmt.Errorf("%w: token %s has an invalid category", domain.ErrValidation, token.Name)
		}
	}
	canonical, err := json.Marshal(spec)
	if err != nil {
		return Spec{}, nil, err
	}
	return spec, canonical, nil
}

// Create stores version 1 of a standard.
func (s *Service) Create(ctx context.Context, orgID, actorID, name string, raw []byte) (Standard, error) {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 80 {
		return Standard{}, fmt.Errorf("%w: name must be 3-80 characters", domain.ErrValidation)
	}
	_, canonical, err := Validate(raw)
	if err != nil {
		return Standard{}, err
	}
	sum := sha256.Sum256(canonical)
	created, err := s.store.Create(ctx, orgID, name, 1, canonical, sum[:])
	if err != nil {
		return Standard{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "design_standard.created", created.ID, nil)
	return created, nil
}

// CreateVersion clones or replaces the document into the next draft version.
func (s *Service) CreateVersion(ctx context.Context, orgID, actorID, id string, raw []byte) (Standard, error) {
	current, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return Standard{}, err
	}
	doc := raw
	if len(doc) == 0 {
		doc = current.Spec
	}
	_, canonical, err := Validate(doc)
	if err != nil {
		return Standard{}, err
	}
	version, err := s.store.NextVersion(ctx, orgID, current.Name)
	if err != nil {
		return Standard{}, err
	}
	sum := sha256.Sum256(canonical)
	created, err := s.store.Create(ctx, orgID, current.Name, version, canonical, sum[:])
	if err != nil {
		return Standard{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "design_standard.version_created", created.ID, nil)
	return created, nil
}

// Publish freezes a draft.
func (s *Service) Publish(ctx context.Context, orgID, actorID, id string) (Standard, error) {
	published, err := s.store.Publish(ctx, orgID, id)
	if err != nil {
		return Standard{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "design_standard.published", id, nil)
	return published, nil
}

// List returns the org's standards.
func (s *Service) List(ctx context.Context, orgID string, limit int) ([]Standard, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.List(ctx, orgID, limit)
}

// Get returns one standard.
func (s *Service) Get(ctx context.Context, orgID, id string) (Standard, error) {
	return s.store.Get(ctx, orgID, id)
}
