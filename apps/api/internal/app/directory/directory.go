// Package directory lists workspace-wide resources: repos, runs, members, audit.
package directory

import (
	"context"
	"fmt"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Repo is a connected repository with its project name.
type Repo struct {
	ID            string
	ProjectID     string
	ProjectName   string
	Provider      string
	Name          string
	DefaultBranch string
	CreatedAt     time.Time
}

// Run is an org-wide run row.
type Run struct {
	ID         string
	ProjectID  string
	TaskRef    string
	TaskTitle  string
	AgentType  string
	Status     string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// Member is a workspace member.
type Member struct {
	UserID    string
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
}

// AuditEntry is one append-only audit record.
type AuditEntry struct {
	ID        int64
	ActorID   string
	ActorKind string
	Action    string
	Target    string
	Metadata  []byte
	PrevHash  string
	Hash      string
	CreatedAt time.Time
}

// Store reads the directory data.
type Store interface {
	Repos(ctx context.Context, orgID string, limit int) ([]Repo, error)
	Runs(ctx context.Context, orgID, status, agentType string, limit int) ([]Run, error)
	Members(ctx context.Context, orgID string, limit int) ([]Member, error)
	Audit(ctx context.Context, orgID string, limit int) ([]AuditEntry, error)
	UpdateRole(ctx context.Context, orgID, userID, role string) error
	RemoveMember(ctx context.Context, orgID, userID string) error
	RecordAudit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

var roles = map[string]bool{"admin": true, "designer": true, "member": true, "viewer": true}

// UpdateRole changes a member's role. The owner role is immutable.
func (s *Service) UpdateRole(ctx context.Context, orgID, actorID, userID, role string) error {
	if !roles[role] {
		return fmt.Errorf("%w: role must be admin, member or viewer", domain.ErrValidation)
	}
	member, err := s.member(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if member.Role == "owner" {
		return fmt.Errorf("%w: the owner role cannot be changed", domain.ErrValidation)
	}
	if err := s.store.UpdateRole(ctx, orgID, userID, role); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, orgID, actorID, "member.role_changed", userID,
		[]byte(fmt.Sprintf(`{"from":%q,"to":%q}`, member.Role, role)))
	return nil
}

// RemoveMember removes a member. Owners and the actor themselves are protected.
func (s *Service) RemoveMember(ctx context.Context, orgID, actorID, userID string) error {
	if userID == actorID {
		return fmt.Errorf("%w: you cannot remove yourself", domain.ErrValidation)
	}
	member, err := s.member(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if member.Role == "owner" {
		return fmt.Errorf("%w: the owner cannot be removed", domain.ErrValidation)
	}
	if err := s.store.RemoveMember(ctx, orgID, userID); err != nil {
		return err
	}
	_ = s.store.RecordAudit(ctx, orgID, actorID, "member.removed", userID, []byte(`{}`))
	return nil
}

func (s *Service) member(ctx context.Context, orgID, userID string) (Member, error) {
	members, err := s.store.Members(ctx, orgID, 200)
	if err != nil {
		return Member{}, err
	}
	for _, member := range members {
		if member.UserID == userID {
			return member, nil
		}
	}
	return Member{}, domain.ErrNotFound
}

// Service clamps limits and exposes the store.
type Service struct{ store Store }

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

func clamp(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}

// Repos lists connected repositories.
func (s *Service) Repos(ctx context.Context, orgID string, limit int) ([]Repo, error) {
	return s.store.Repos(ctx, orgID, clamp(limit))
}

// Runs lists runs, optionally filtered by status and agent type.
func (s *Service) Runs(ctx context.Context, orgID, status, agentType string, limit int) ([]Run, error) {
	return s.store.Runs(ctx, orgID, status, agentType, clamp(limit))
}

// Members lists workspace members.
func (s *Service) Members(ctx context.Context, orgID string, limit int) ([]Member, error) {
	return s.store.Members(ctx, orgID, clamp(limit))
}

// Audit lists the audit log, newest first.
func (s *Service) Audit(ctx context.Context, orgID string, limit int) ([]AuditEntry, error) {
	return s.store.Audit(ctx, orgID, clamp(limit))
}
