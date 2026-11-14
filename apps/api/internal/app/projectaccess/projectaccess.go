// Package projectaccess manages per-project roles and access checks.
package projectaccess

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Member is one project role.
type Member struct {
	UserID    string
	Role      string
	CreatedAt time.Time
}

// Access is the effective project access for one user.
type Access struct {
	Role    string
	Members int
}

// Store persists project roles.
type Store interface {
	Upsert(ctx context.Context, orgID, projectID, userID, role string) (Member, error)
	List(ctx context.Context, orgID, projectID string) ([]Member, error)
	Delete(ctx context.Context, orgID, projectID, userID string) error
	Access(ctx context.Context, orgID, projectID, userID string) (Access, error)
}

// Service validates roles.
type Service struct{ store Store }

// New wires the service.
func New(store Store) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("projectaccess: store is required")
	}
	return &Service{store: store}, nil
}

var roles = map[string]bool{"admin": true, "designer": true, "member": true, "viewer": true}

// SetRole assigns a project role.
func (s *Service) SetRole(ctx context.Context, orgID, projectID, userID, role string) (Member, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if !roles[role] {
		return Member{}, fmt.Errorf("%w: role must be admin, designer, member or viewer", domain.ErrValidation)
	}
	return s.store.Upsert(ctx, orgID, projectID, userID, role)
}

// List returns project members.
func (s *Service) List(ctx context.Context, orgID, projectID string) ([]Member, error) {
	return s.store.List(ctx, orgID, projectID)
}

// Delete removes a project role.
func (s *Service) Delete(ctx context.Context, orgID, projectID, userID string) error {
	return s.store.Delete(ctx, orgID, projectID, userID)
}

// Access returns the effective project access.
func (s *Service) Access(ctx context.Context, orgID, projectID, userID string) (Access, error) {
	return s.store.Access(ctx, orgID, projectID, userID)
}
