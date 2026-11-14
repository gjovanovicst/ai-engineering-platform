// Package rules manages versioned engineering rules that are injected into
// prompts (docs/03 §4: rules -> rule_versions).
package rules

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Rule is a named rule at a level.
type Rule struct {
	ID        string
	OrgID     string
	ProjectID string
	Level     string
	ScopeRef  string
	Name      string
	CreatedAt time.Time
}

// Version is one immutable revision of a rule body.
type Version struct {
	ID          string
	RuleID      string
	Version     int
	Body        string
	SHA256      []byte
	PublishedAt *time.Time
	PublishedBy string
	CreatedAt   time.Time
}

// Active is a published rule version used for prompts.
type Active struct {
	RuleID  string
	Name    string
	Level   string
	Version int
	Body    string
	SHA256  []byte
}

// Store persists rules and versions.
type Store interface {
	CreateRule(ctx context.Context, r Rule) (Rule, error)
	ListRules(ctx context.Context, orgID, projectID string, limit int) ([]Rule, error)
	GetRule(ctx context.Context, orgID, id string) (Rule, error)
	CreateVersion(ctx context.Context, orgID, ruleID, body string, sum []byte) (Version, error)
	ListVersions(ctx context.Context, orgID, ruleID string) ([]Version, error)
	PublishVersion(ctx context.Context, orgID, ruleID string, version int, userID string) (Version, error)
	Active(ctx context.Context, orgID, projectID string) ([]Active, error)
	GetVersion(ctx context.Context, orgID, id string) (Version, error)
	CreatePublishApproval(ctx context.Context, orgID, versionID, requestedBy string, expires time.Time) (PublishRequest, error)
	PendingPublishes(ctx context.Context, orgID string) ([]PublishRequest, error)
	GetPublishApproval(ctx context.Context, orgID, id string) (PublishRequest, error)
	DecidePublishApproval(ctx context.Context, orgID, id, status, userID, reason string) error
}

// Service validates and versions rules.
type Service struct {
	store Store
	log   *slog.Logger
}

// New wires the service.
func New(store Store, log *slog.Logger) (*Service, error) {
	if store == nil {
		return nil, errors.New("rules: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, log: log}, nil
}

// CreateInput creates a rule and its first draft version.
type CreateInput struct {
	OrgID     string
	ProjectID string
	Level     string
	ScopeRef  string
	Name      string
	Body      string
}

var levels = map[string]bool{"organization": true, "project": true, "module": true, "task": true}

// Create stores a rule with version 1 (unpublished).
func (s *Service) Create(ctx context.Context, in CreateInput) (Rule, Version, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Rule{}, Version{}, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	if !levels[in.Level] {
		return Rule{}, Version{}, fmt.Errorf("%w: unknown rule level", domain.ErrValidation)
	}
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return Rule{}, Version{}, fmt.Errorf("%w: body is required", domain.ErrValidation)
	}
	rule, err := s.store.CreateRule(ctx, Rule{
		OrgID: in.OrgID, ProjectID: in.ProjectID, Level: in.Level, ScopeRef: in.ScopeRef, Name: name,
	})
	if err != nil {
		return Rule{}, Version{}, err
	}
	sum := sha256.Sum256([]byte(body))
	version, err := s.store.CreateVersion(ctx, in.OrgID, rule.ID, body, sum[:])
	if err != nil {
		return Rule{}, Version{}, err
	}
	return rule, version, nil
}

// List returns the org rules.
func (s *Service) List(ctx context.Context, orgID, projectID string, limit int) ([]Rule, error) {
	return s.store.ListRules(ctx, orgID, projectID, limit)
}

// Versions returns the versions of a rule.
func (s *Service) Versions(ctx context.Context, orgID, ruleID string) ([]Version, error) {
	if _, err := s.store.GetRule(ctx, orgID, ruleID); err != nil {
		return nil, err
	}
	return s.store.ListVersions(ctx, orgID, ruleID)
}

// SaveDraft appends a new unpublished version.
func (s *Service) SaveDraft(ctx context.Context, orgID, ruleID, body string) (Version, error) {
	if _, err := s.store.GetRule(ctx, orgID, ruleID); err != nil {
		return Version{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return Version{}, fmt.Errorf("%w: body is required", domain.ErrValidation)
	}
	sum := sha256.Sum256([]byte(body))
	return s.store.CreateVersion(ctx, orgID, ruleID, body, sum[:])
}

// Publish marks a version as active; the audit trail is in the version row.
// PublishRequest is a pending rule_publish approval.
type PublishRequest struct {
	ID          string
	RuleID      string
	RuleName    string
	Version     int
	RequestedBy string
}

// RequestPublish asks a second admin to publish a draft.
func (s *Service) RequestPublish(ctx context.Context, orgID, ruleID string, version int, userID string) (PublishRequest, error) {
	if _, err := s.store.GetRule(ctx, orgID, ruleID); err != nil {
		return PublishRequest{}, err
	}
	versions, err := s.store.ListVersions(ctx, orgID, ruleID)
	if err != nil {
		return PublishRequest{}, err
	}
	var target *Version
	for i := range versions {
		if versions[i].Version == version {
			target = &versions[i]
			break
		}
	}
	if target == nil {
		return PublishRequest{}, fmt.Errorf("%w: version %d does not exist", domain.ErrNotFound, version)
	}
	if target.PublishedAt != nil {
		return PublishRequest{}, fmt.Errorf("%w: this version is already active", domain.ErrValidation)
	}
	return s.store.CreatePublishApproval(ctx, orgID, target.ID, userID, time.Now().UTC().Add(48*time.Hour))
}

// PendingPublishes lists rule versions waiting for approval.
func (s *Service) PendingPublishes(ctx context.Context, orgID string) ([]PublishRequest, error) {
	return s.store.PendingPublishes(ctx, orgID)
}

// DecidePublish approves or rejects a publish request. The approver must differ
// from the requester; the handler enforces the admin role.
func (s *Service) DecidePublish(ctx context.Context, orgID, approvalID, decision, userID, reason string) (Version, error) {
	if decision != "approved" && decision != "rejected" {
		return Version{}, fmt.Errorf("%w: decision must be approved or rejected", domain.ErrValidation)
	}
	approval, err := s.store.GetPublishApproval(ctx, orgID, approvalID)
	if err != nil {
		return Version{}, err
	}
	if approval.RequestedBy == userID {
		return Version{}, fmt.Errorf("%w: the approver must be a different admin", domain.ErrValidation)
	}
	var published Version
	if decision == "approved" {
		published, err = s.store.PublishVersion(ctx, orgID, approval.RuleID, approval.Version, userID)
		if err != nil {
			return Version{}, err
		}
	}
	if err := s.store.DecidePublishApproval(ctx, orgID, approvalID, decision, userID, reason); err != nil {
		return Version{}, err
	}
	return published, nil
}

func (s *Service) Publish(ctx context.Context, orgID, ruleID string, version int, userID string) (Version, error) {
	if _, err := s.store.GetRule(ctx, orgID, ruleID); err != nil {
		return Version{}, err
	}
	return s.store.PublishVersion(ctx, orgID, ruleID, version, userID)
}

// Ref identifies a rule version for audit.
type Ref struct {
	RuleID  string `json:"rule_id"`
	Name    string `json:"name"`
	Level   string `json:"level"`
	Version int    `json:"version"`
}

// PromptText returns the published rules as prompt text plus their refs.
func (s *Service) PromptText(ctx context.Context, orgID, projectID string) (string, []Ref, error) {
	active, err := s.store.Active(ctx, orgID, projectID)
	if err != nil {
		return "", nil, err
	}
	if len(active) == 0 {
		return "", nil, nil
	}
	var b strings.Builder
	b.WriteString("Engineering rules (follow them strictly):\n")
	refs := make([]Ref, 0, len(active))
	for _, rule := range active {
		fmt.Fprintf(&b, "\n### %s [%s v%d]\n%s\n", rule.Name, rule.Level, rule.Version, rule.Body)
		refs = append(refs, Ref{RuleID: rule.RuleID, Name: rule.Name, Level: rule.Level, Version: rule.Version})
	}
	return b.String(), refs, nil
}
