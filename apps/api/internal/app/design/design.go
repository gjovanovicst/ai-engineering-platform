// Package design manages the org design-token and component registry.
package design

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Token is a design token override.
type Token struct {
	ID       string
	Name     string
	Value    string
	Category string
}

// Component is a registered component contract.
type Component struct {
	ID         string
	Name       string
	Purpose    string
	Rules      string
	Status     string
	Category   string
	Framework  string
	Package    string
	ImportPath string
	Version    string
	Docs       string
	Example    string
}

// ForbiddenComponent is a deprecated/forbidden import path.
type ForbiddenComponent struct {
	Name       string
	ImportPath string
}

// Store persists tokens and components.
type Store interface {
	ListTokens(ctx context.Context, orgID string) ([]Token, error)
	CreateToken(ctx context.Context, orgID string, t Token) (Token, error)
	UpdateToken(ctx context.Context, orgID, id string, t Token) (Token, error)
	DeleteToken(ctx context.Context, orgID, id string) error
	CountTokens(ctx context.Context, orgID string) (int, error)
	ListComponents(ctx context.Context, orgID string) ([]Component, error)
	CreateComponent(ctx context.Context, orgID string, c Component) (Component, error)
	UpdateComponent(ctx context.Context, orgID, id string, c Component) (Component, error)
	DeleteComponent(ctx context.Context, orgID, id string) error
	CountComponents(ctx context.Context, orgID string) (int, error)
}

// Service validates registry writes.
type Service struct {
	store Store
	log   *slog.Logger
}

// New wires the service.
func New(store Store, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, log: log}
}

var categories = map[string]bool{"color": true, "typography": true, "spacing": true, "radius": true, "layout": true, "other": true}
var statuses = map[string]bool{"draft": true, "published": true, "approved": true, "deprecated": true, "forbidden": true}
var componentCategories = map[string]bool{"layout": true, "navigation": true, "data": true, "form": true, "feedback": true, "overlay": true, "typography": true, "other": true}

// ListTokens returns the org tokens.
func (s *Service) ListTokens(ctx context.Context, orgID string) ([]Token, error) {
	return s.store.ListTokens(ctx, orgID)
}

// CreateToken validates and stores a token.
func (s *Service) CreateToken(ctx context.Context, orgID string, t Token) (Token, error) {
	if err := validateToken(t); err != nil {
		return Token{}, err
	}
	if t.Category == "" {
		t.Category = "other"
	}
	return s.store.CreateToken(ctx, orgID, t)
}

// UpdateToken changes the value and category of a token (the name is immutable).
func (s *Service) UpdateToken(ctx context.Context, orgID, id string, t Token) (Token, error) {
	if strings.TrimSpace(t.Value) == "" {
		return Token{}, fmt.Errorf("%w: token value is required", domain.ErrValidation)
	}
	if t.Category != "" && !categories[t.Category] {
		return Token{}, fmt.Errorf("%w: unknown token category", domain.ErrValidation)
	}
	if t.Category == "" {
		t.Category = "other"
	}
	return s.store.UpdateToken(ctx, orgID, id, t)
}

// DeleteToken removes a token.
func (s *Service) DeleteToken(ctx context.Context, orgID, id string) error {
	return s.store.DeleteToken(ctx, orgID, id)
}

// ListComponents returns the org components.
func (s *Service) ListComponents(ctx context.Context, orgID string) ([]Component, error) {
	return s.store.ListComponents(ctx, orgID)
}

// CreateComponent validates and stores a component.
func (s *Service) CreateComponent(ctx context.Context, orgID string, c Component) (Component, error) {
	if err := validateComponent(c); err != nil {
		return Component{}, err
	}
	return s.store.CreateComponent(ctx, orgID, c)
}

// UpdateComponent changes an entry.
func (s *Service) UpdateComponent(ctx context.Context, orgID, id string, c Component) (Component, error) {
	if err := validateComponent(c); err != nil {
		return Component{}, err
	}
	return s.store.UpdateComponent(ctx, orgID, id, c)
}

// DeleteComponent removes an entry.
func (s *Service) DeleteComponent(ctx context.Context, orgID, id string) error {
	return s.store.DeleteComponent(ctx, orgID, id)
}

func validateToken(t Token) error {
	if strings.TrimSpace(t.Name) == "" || len(t.Name) > 80 {
		return fmt.Errorf("%w: token name is required (max 80)", domain.ErrValidation)
	}
	if strings.TrimSpace(t.Value) == "" {
		return fmt.Errorf("%w: token value is required", domain.ErrValidation)
	}
	if t.Category != "" && !categories[t.Category] {
		return fmt.Errorf("%w: unknown token category", domain.ErrValidation)
	}
	return nil
}

func validateComponent(c Component) error {
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 80 {
		return fmt.Errorf("%w: component name is required (max 80)", domain.ErrValidation)
	}
	if c.Status == "" {
		c.Status = "draft"
	}
	if !statuses[c.Status] {
		return fmt.Errorf("%w: unknown component status", domain.ErrValidation)
	}
	if c.Category == "" {
		c.Category = "other"
	}
	if !componentCategories[c.Category] {
		return fmt.Errorf("%w: unknown component category", domain.ErrValidation)
	}
	return nil
}

// SeedDefaults inserts the built-in baseline when the registry is empty.
func (s *Service) SeedDefaults(ctx context.Context, orgID string) (int, int, error) {
	tokens, err := s.store.CountTokens(ctx, orgID)
	if err != nil {
		return 0, 0, err
	}
	components, err := s.store.CountComponents(ctx, orgID)
	if err != nil {
		return 0, 0, err
	}
	createdTokens, createdComponents := 0, 0
	if tokens == 0 {
		for _, t := range DefaultTokens {
			if _, err := s.store.CreateToken(ctx, orgID, t); err == nil {
				createdTokens++
			}
		}
	}
	if components == 0 {
		for _, c := range DefaultComponents {
			if _, err := s.store.CreateComponent(ctx, orgID, c); err == nil {
				createdComponents++
			}
		}
	}
	return createdTokens, createdComponents, nil
}

// DefaultTokens mirrors apps/web/src/design/tokens.json.
var DefaultTokens = []Token{
	{Name: "bg-canvas", Value: "#F8FAFC", Category: "color"},
	{Name: "bg-surface", Value: "#FFFFFF", Category: "color"},
	{Name: "bg-hover", Value: "#F1F5F9", Category: "color"},
	{Name: "text-primary", Value: "#0F172A", Category: "color"},
	{Name: "text-secondary", Value: "#475569", Category: "color"},
	{Name: "text-muted", Value: "#94A3B8", Category: "color"},
	{Name: "brand", Value: "#6366F1", Category: "color"},
	{Name: "brand-hover", Value: "#4F46E5", Category: "color"},
	{Name: "ai", Value: "#8B5CF6", Category: "color"},
	{Name: "status-success", Value: "#10B981", Category: "color"},
	{Name: "status-warning", Value: "#F59E0B", Category: "color"},
	{Name: "status-danger", Value: "#EF4444", Category: "color"},
}

// DefaultComponents mirrors the component contracts in the docs.
var DefaultComponents = []Component{
	{Name: "Button", Purpose: "The only interactive action element. One primary per view.", Rules: "Variants primary/secondary/ghost/danger; loading shows …; default type=button.", Status: "published"},
	{Name: "Input", Purpose: "Single-line text entry, always inside a Field.", Rules: "Never use placeholders as labels; errors announced via role=alert.", Status: "published"},
	{Name: "Field", Purpose: "Label + control + error message.", Rules: "label[for] is required; error text is visible and announced.", Status: "published"},
	{Name: "Status chip", Purpose: "Read-only state (run, task, approval).", Rules: "Colour plus text; never colour alone.", Status: "published"},
	{Name: "Dialog / drawer", Purpose: "Focused task (task drawer, approvals).", Rules: "aria-modal, Escape closes, focus is trapped and restored.", Status: "published"},
	{Name: "Diff viewer", Purpose: "Show exactly what the agent changed.", Rules: "Additions/deletions use colour and a +/- prefix.", Status: "published"},
}

// ImportComponent is one bulk registry entry (Storybook/Figma export).
type ImportComponent struct {
	Name       string `json:"name"`
	ImportPath string `json:"import_path"`
	Package    string `json:"package"`
	Framework  string `json:"framework"`
	Category   string `json:"category"`
	Purpose    string `json:"purpose"`
	Rules      string `json:"rules"`
	Version    string `json:"version"`
}

// ImportComponents registers entries in bulk, skipping duplicate import paths.
func (s *Service) ImportComponents(ctx context.Context, orgID string, items []ImportComponent) (int, int, error) {
	if len(items) == 0 || len(items) > 200 {
		return 0, 0, fmt.Errorf("%w: import 1..200 components", domain.ErrValidation)
	}
	existing, err := s.store.ListComponents(ctx, orgID)
	if err != nil {
		return 0, 0, err
	}
	seen := map[string]bool{}
	for _, component := range existing {
		seen[component.ImportPath] = true
	}
	created, skipped := 0, 0
	for _, item := range items {
		if item.ImportPath != "" && seen[item.ImportPath] {
			skipped++
			continue
		}
		category := item.Category
		if category == "" {
			category = "other"
		}
		component := Component{
			Name: item.Name, ImportPath: item.ImportPath, Package: item.Package, Framework: item.Framework,
			Category: category, Purpose: item.Purpose, Rules: item.Rules, Version: item.Version, Status: "approved",
		}
		if err := validateComponent(component); err != nil {
			return created, skipped, err
		}
		if _, err := s.store.CreateComponent(ctx, orgID, component); err != nil {
			return created, skipped, err
		}
		if item.ImportPath != "" {
			seen[item.ImportPath] = true
		}
		created++
	}
	return created, skipped, nil
}
