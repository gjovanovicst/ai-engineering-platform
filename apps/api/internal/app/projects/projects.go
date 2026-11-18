// Package projects implements project use cases.
package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/policyschedule"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Store is the persistence port.
type Store interface {
	Create(ctx context.Context, p domain.Project) (domain.Project, error)
	Get(ctx context.Context, orgID, id string) (domain.Project, error)
	List(ctx context.Context, orgID string, limit int) ([]domain.Project, error)
	UpdateRequiredChecks(ctx context.Context, orgID, id string, checks []string) (domain.Project, error)
	SetDesignStandard(ctx context.Context, orgID, projectID, standardID string) (domain.Project, error)
	DesignStandard(ctx context.Context, orgID, projectID string) (ProjectStandard, error)
	ListComponents(ctx context.Context, orgID, projectID string) ([]ProjectComponent, error)
	ReplaceComponents(ctx context.Context, orgID, projectID, actorID string, selections []ComponentSelection) (int, error)
	SetE2ECommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	SetTestCommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	SetVisualCommand(ctx context.Context, orgID, id, command string) (domain.Project, error)
	SetMCPConfig(ctx context.Context, orgID, id, config string) (domain.Project, error)
	SetVisualPolicy(ctx context.Context, orgID, id string, threshold float64, masks string) (domain.Project, error)
	SetPolicy(ctx context.Context, orgID, id string, maxBytes int, paths []string) (domain.Project, error)
	SetChangeWindow(ctx context.Context, orgID, id, window string) (domain.Project, error)
	SetAllowedModels(ctx context.Context, orgID, id string, models []string) (domain.Project, error)
	SetAllowedTools(ctx context.Context, orgID, id string, tools []string) (domain.Project, error)
	SetRequiredApprovals(ctx context.Context, orgID, id string, n int) (domain.Project, error)
	SetPluginChecks(ctx context.Context, orgID, id, checks string) (domain.Project, error)
	Delete(ctx context.Context, orgID, id string) error
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// ComponentSelection is one allowlist entry.
type ComponentSelection struct {
	ID     string
	Mode   string
	Reason string
}

// ProjectComponent is an approved component allowed in a project.
type ProjectComponent struct {
	ID         string
	Name       string
	Category   string
	Status     string
	ImportPath string
	Package    string
	Version    string
	Mode       string
	Reason     string
}

// ProjectStandard is the published design standard a project inherits.
type ProjectStandard struct {
	ID          string
	Name        string
	Version     int
	SHA256      string
	PublishedAt *time.Time
}

// Service holds the project use cases.
type Service struct {
	store Store
}

// New returns a project service.
func New(store Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("projects: store is required")
	}
	return &Service{store: store}, nil
}

// CreateInput is the create-project payload.
type CreateInput struct {
	OrgID       string
	Name        string
	Slug        string
	Description string
	BlueprintID string
}

// Create validates and stores a project. The slug is derived from the name
// when the caller does not provide one.
func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Project, error) {
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = domain.Slugify(in.Name)
	}
	p, err := domain.NewProject(in.OrgID, in.Name, slug)
	if err != nil {
		return domain.Project{}, err
	}
	if len(in.Description) > 5000 {
		return domain.Project{}, fmt.Errorf("%w: description must be at most 5000 characters", domain.ErrValidation)
	}
	p.Description = strings.TrimSpace(in.Description)
	p.BlueprintID = in.BlueprintID
	return s.store.Create(ctx, *p)
}

// UpdateRequiredChecks sets the CI checks that block a merge for this project.
func (s *Service) UpdateRequiredChecks(ctx context.Context, orgID, id string, checks []string) (domain.Project, error) {
	seen := map[string]bool{}
	clean := make([]string, 0, len(checks))
	for _, name := range checks {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if len(name) > 100 {
			return domain.Project{}, fmt.Errorf("%w: check name is too long", domain.ErrValidation)
		}
		if !seen[name] {
			seen[name] = true
			clean = append(clean, name)
		}
	}
	if len(clean) > 20 {
		return domain.Project{}, fmt.Errorf("%w: at most 20 required checks", domain.ErrValidation)
	}
	return s.store.UpdateRequiredChecks(ctx, orgID, id, clean)
}

// Get returns one project.
func (s *Service) Get(ctx context.Context, orgID, id string) (domain.Project, error) {
	return s.store.Get(ctx, orgID, id)
}

// List returns up to limit projects (default 50, max 100).
func (s *Service) List(ctx context.Context, orgID string, limit int) ([]domain.Project, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.List(ctx, orgID, limit)
}

// SetDesignStandard assigns a published standard (empty clears it).
func (s *Service) SetDesignStandard(ctx context.Context, orgID, projectID, standardID string) (domain.Project, error) {
	if standardID == "" {
		return s.store.SetDesignStandard(ctx, orgID, projectID, "")
	}
	return s.store.SetDesignStandard(ctx, orgID, projectID, standardID)
}

// DesignStandard returns the standard assigned to a project.
func (s *Service) DesignStandard(ctx context.Context, orgID, projectID string) (ProjectStandard, error) {
	return s.store.DesignStandard(ctx, orgID, projectID)
}

// ListComponents returns the project's component allowlist.
func (s *Service) ListComponents(ctx context.Context, orgID, projectID string) ([]ProjectComponent, error) {
	return s.store.ListComponents(ctx, orgID, projectID)
}

// ReplaceComponents sets the allowlist; only approved/published components are accepted.
func (s *Service) ReplaceComponents(ctx context.Context, orgID, projectID, actorID string, selections []ComponentSelection) (int, error) {
	if len(selections) > 100 {
		return 0, fmt.Errorf("%w: at most 100 components", domain.ErrValidation)
	}
	for i := range selections {
		if selections[i].Mode == "" {
			selections[i].Mode = "allow"
		}
		if selections[i].Mode != "allow" && selections[i].Mode != "exception" {
			return 0, fmt.Errorf("%w: mode must be allow or exception", domain.ErrValidation)
		}
		selections[i].Reason = strings.TrimSpace(selections[i].Reason)
		if selections[i].Mode == "exception" && len(selections[i].Reason) < 5 {
			return 0, fmt.Errorf("%w: an exception needs a reason (min 5 characters)", domain.ErrValidation)
		}
		if len(selections[i].Reason) > 500 {
			return 0, fmt.Errorf("%w: reason is too long", domain.ErrValidation)
		}
	}
	inserted, err := s.store.ReplaceComponents(ctx, orgID, projectID, actorID, selections)
	if err != nil {
		return 0, err
	}
	if inserted != len(selections) {
		return inserted, fmt.Errorf("%w: every component must exist and be approved/published", domain.ErrValidation)
	}
	return inserted, nil
}

// SetE2ECommand stores the project e2e command.
func (s *Service) SetE2ECommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	command = strings.TrimSpace(command)
	if len(command) > 500 {
		return domain.Project{}, fmt.Errorf("%w: the e2e command is too long", domain.ErrValidation)
	}
	return s.store.SetE2ECommand(ctx, orgID, id, command)
}

// SetTestCommand stores the project test command (empty means the global one).
func (s *Service) SetTestCommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	command = strings.TrimSpace(command)
	if len(command) > 500 {
		return domain.Project{}, fmt.Errorf("%w: the test command is too long", domain.ErrValidation)
	}
	return s.store.SetTestCommand(ctx, orgID, id, command)
}

// SetVisualCommand stores the project visual diff command.
func (s *Service) SetVisualCommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	command = strings.TrimSpace(command)
	if len(command) > 500 {
		return domain.Project{}, fmt.Errorf("%w: the visual command is too long", domain.ErrValidation)
	}
	return s.store.SetVisualCommand(ctx, orgID, id, command)
}

// SetMCPConfig stores and validates the project MCP server JSON.
func (s *Service) SetMCPConfig(ctx context.Context, orgID, id, config string) (domain.Project, error) {
	config = strings.TrimSpace(config)
	if len(config) > 20000 {
		return domain.Project{}, fmt.Errorf("%w: the MCP config is too long", domain.ErrValidation)
	}
	if config != "" {
		var servers []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(config), &servers); err != nil {
			return domain.Project{}, fmt.Errorf("%w: invalid MCP JSON: %w", domain.ErrValidation, err)
		}
		for _, server := range servers {
			if strings.TrimSpace(server.Name) == "" || strings.TrimSpace(server.Command) == "" {
				return domain.Project{}, fmt.Errorf("%w: every MCP server needs name and command", domain.ErrValidation)
			}
		}
	}
	return s.store.SetMCPConfig(ctx, orgID, id, config)
}

// SetVisualPolicy validates and stores the screenshot threshold and masks.
func (s *Service) SetVisualPolicy(ctx context.Context, orgID, id string, threshold float64, masks string) (domain.Project, error) {
	if threshold < 0 || threshold > 1 {
		return domain.Project{}, fmt.Errorf("%w: threshold must be between 0 and 1", domain.ErrValidation)
	}
	masks = strings.TrimSpace(masks)
	if masks == "" {
		masks = "[]"
	}
	if len(masks) > 10000 {
		return domain.Project{}, fmt.Errorf("%w: masks are too large", domain.ErrValidation)
	}
	var parsed []struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	}
	if err := json.Unmarshal([]byte(masks), &parsed); err != nil {
		return domain.Project{}, fmt.Errorf("%w: invalid masks JSON: %w", domain.ErrValidation, err)
	}
	for _, mask := range parsed {
		if mask.W <= 0 || mask.H <= 0 || mask.X < 0 || mask.Y < 0 {
			return domain.Project{}, fmt.Errorf("%w: every mask needs x/y >= 0 and w/h > 0", domain.ErrValidation)
		}
	}
	return s.store.SetVisualPolicy(ctx, orgID, id, threshold, masks)
}

// SetPolicy validates and stores the project diff policy.
func (s *Service) SetPolicy(ctx context.Context, orgID, id string, maxBytes int, paths []string) (domain.Project, error) {
	if maxBytes < 0 || maxBytes > 5<<20 {
		return domain.Project{}, fmt.Errorf("%w: max diff must be 0..5 MiB (0 disables)", domain.ErrValidation)
	}
	if len(paths) > 50 {
		return domain.Project{}, fmt.Errorf("%w: at most 50 forbidden paths", domain.ErrValidation)
	}
	clean := make([]string, 0, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(strings.TrimPrefix(raw, "./"))
		if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
			return domain.Project{}, fmt.Errorf("%w: forbidden paths must be relative and stay inside the repo", domain.ErrValidation)
		}
		clean = append(clean, path)
	}
	return s.store.SetPolicy(ctx, orgID, id, maxBytes, clean)
}

// SetChangeWindow validates and stores the merge window.
func (s *Service) SetChangeWindow(ctx context.Context, orgID, id, window string) (domain.Project, error) {
	window = strings.TrimSpace(window)
	if window != "" {
		if _, err := policyschedule.Parse(window); err != nil {
			return domain.Project{}, fmt.Errorf("%w: %w", domain.ErrValidation, err)
		}
	}
	return s.store.SetChangeWindow(ctx, orgID, id, window)
}

// ChangeWindow returns the project merge window JSON (empty = always).
func (s *Service) ChangeWindow(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.PolicyChangeWindow, nil
}

// SetAllowedModels validates and stores the model allowlist.
func (s *Service) SetAllowedModels(ctx context.Context, orgID, id string, models []string) (domain.Project, error) {
	if len(models) > 20 {
		return domain.Project{}, fmt.Errorf("%w: at most 20 allowed models", domain.ErrValidation)
	}
	clean := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, raw := range models {
		model := strings.TrimSpace(raw)
		if model == "" || len(model) > 100 {
			return domain.Project{}, fmt.Errorf("%w: model names must be 1..100 characters", domain.ErrValidation)
		}
		if !seen[model] {
			seen[model] = true
			clean = append(clean, model)
		}
	}
	return s.store.SetAllowedModels(ctx, orgID, id, clean)
}

// SetAllowedTools validates and stores the MCP tool allowlist.
func (s *Service) SetAllowedTools(ctx context.Context, orgID, id string, tools []string) (domain.Project, error) {
	if len(tools) > 50 {
		return domain.Project{}, fmt.Errorf("%w: at most 50 allowed tools", domain.ErrValidation)
	}
	clean := make([]string, 0, len(tools))
	seen := map[string]bool{}
	for _, raw := range tools {
		pattern := strings.TrimSpace(raw)
		if !strings.HasPrefix(pattern, "mcp__") || len(pattern) > 200 {
			return domain.Project{}, fmt.Errorf("%w: tool patterns must start with mcp__", domain.ErrValidation)
		}
		if !seen[pattern] {
			seen[pattern] = true
			clean = append(clean, pattern)
		}
	}
	return s.store.SetAllowedTools(ctx, orgID, id, clean)
}

// SetRequiredApprovals validates and stores the approval threshold.
func (s *Service) SetRequiredApprovals(ctx context.Context, orgID, id string, n int) (domain.Project, error) {
	if n < 1 || n > 3 {
		return domain.Project{}, fmt.Errorf("%w: required approvals must be 1..3", domain.ErrValidation)
	}
	return s.store.SetRequiredApprovals(ctx, orgID, id, n)
}

// RequiredApprovals returns how many approvals a PR merge needs (default 1).
func (s *Service) RequiredApprovals(ctx context.Context, orgID, projectID string) (int, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return 0, err
	}
	return project.PolicyRequiredApprovals, nil
}

// SetPluginChecks validates the JSON list of named sandbox commands.
func (s *Service) SetPluginChecks(ctx context.Context, orgID, id, raw string) (domain.Project, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "[]"
	}
	if len(raw) > 20000 {
		return domain.Project{}, fmt.Errorf("%w: plugin checks are too large", domain.ErrValidation)
	}
	var checks []struct {
		Name    string `json:"name"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		return domain.Project{}, fmt.Errorf("%w: invalid JSON: %w", domain.ErrValidation, err)
	}
	if len(checks) > 5 {
		return domain.Project{}, fmt.Errorf("%w: at most 5 plugin checks", domain.ErrValidation)
	}
	for _, check := range checks {
		if strings.TrimSpace(check.Name) == "" || len(check.Name) > 60 || strings.TrimSpace(check.Command) == "" || len(check.Command) > 500 {
			return domain.Project{}, fmt.Errorf("%w: every plugin check needs a name (<=60) and command (<=500)", domain.ErrValidation)
		}
	}
	return s.store.SetPluginChecks(ctx, orgID, id, raw)
}

// Delete removes a project after the caller repeats its name.
func (s *Service) Delete(ctx context.Context, orgID, id, actorID, confirmName string) error {
	project, err := s.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirmName) != project.Name {
		return fmt.Errorf("%w: type the project name %q to confirm", domain.ErrValidation, project.Name)
	}
	if err := s.store.Audit(ctx, orgID, actorID, "project.deleted", project.ID, []byte(`{}`)); err != nil {
		return err
	}
	return s.store.Delete(ctx, orgID, id)
}
