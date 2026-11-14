// Package blueprints validates and instantiates project blueprints
// (docs/09-blueprint-schema.md).
package blueprints

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Blueprint is an immutable project template version.
type Blueprint struct {
	ID          string
	OrgID       string
	Name        string
	Version     int
	Status      string
	PublishedAt *time.Time
	Spec        json.RawMessage
	CreatedAt   time.Time
}

// Store persists blueprints.
type Store interface {
	Create(ctx context.Context, b Blueprint) (Blueprint, error)
	List(ctx context.Context, orgID string, limit int) ([]Blueprint, error)
	Get(ctx context.Context, orgID, id string) (Blueprint, error)
	NextVersion(ctx context.Context, orgID, name string) (int, error)
	CreateVersion(ctx context.Context, orgID, name string, version int, spec []byte) (Blueprint, error)
	Publish(ctx context.Context, orgID, id string) (Blueprint, error)
}

// ProjectCreator creates the project from a blueprint.
type ProjectCreator func(ctx context.Context, orgID, blueprintID, name, description string) (domain.Project, error)

// TaskCreator creates one starter task.
type TaskCreator func(ctx context.Context, orgID, projectID, title, description string, priority domain.Priority, acceptance []domain.AcceptanceCriterion) (domain.Task, error)

// Service validates and instantiates blueprints.
type Service struct {
	store  Store
	create ProjectCreator
	tasks  TaskCreator
	log    *slog.Logger
}

// New wires the service.
func New(store Store, create ProjectCreator, tasks TaskCreator, log *slog.Logger) (*Service, error) {
	if store == nil || create == nil {
		return nil, errors.New("blueprints: store and project creator are required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, create: create, tasks: tasks, log: log}, nil
}

// Create validates the spec and stores version 1.
func (s *Service) Create(ctx context.Context, orgID, name string, spec json.RawMessage) (Blueprint, error) {
	if _, err := Parse(spec); err != nil {
		return Blueprint{}, err
	}
	return s.store.Create(ctx, Blueprint{OrgID: orgID, Name: name, Spec: spec})
}

// List returns the org blueprints.
func (s *Service) List(ctx context.Context, orgID string, limit int) ([]Blueprint, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.List(ctx, orgID, limit)
}

// Instantiate creates a project from a blueprint.
func (s *Service) Instantiate(ctx context.Context, orgID, id string) (domain.Project, int, error) {
	blueprint, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return domain.Project{}, 0, err
	}
	if blueprint.Status != "published" {
		return domain.Project{}, 0, &domain.TransitionError{
			From: domain.TaskStatus(blueprint.Status), To: "published",
			Detail: "only published blueprints can create a project",
		}
	}
	parsed, err := Parse(blueprint.Spec)
	if err != nil {
		return domain.Project{}, 0, err
	}
	project, err := s.create(ctx, orgID, id, parsed.Name, parsed.Description)
	if err != nil {
		return domain.Project{}, 0, err
	}

	created := 0
	if s.tasks != nil {
		for _, starter := range parsed.StarterTasks {
			acceptance := make([]domain.AcceptanceCriterion, 0, len(starter.Acceptance))
			for _, ac := range starter.Acceptance {
				acceptance = append(acceptance, domain.AcceptanceCriterion{Given: ac.Given, When: ac.When, Then: ac.Then})
			}
			if _, terr := s.tasks(ctx, orgID, project.ID, starter.Title, starter.Description, priorityOf(starter.Priority), acceptance); terr != nil {
				s.log.Warn("blueprints: starter task", "err", terr, "title", starter.Title)
				continue
			}
			created++
		}
	}
	return project, created, nil
}

func priorityOf(raw string) domain.Priority {
	switch raw {
	case "p0":
		return domain.PriorityHigh
	case "p1":
		return domain.PriorityMed
	default:
		return domain.PriorityLow
	}
}

// CreateVersion clones a blueprint into the next draft version.
func (s *Service) CreateVersion(ctx context.Context, orgID, id string, spec json.RawMessage) (Blueprint, error) {
	current, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return Blueprint{}, err
	}
	if len(spec) == 0 {
		spec = current.Spec
	}
	if _, err := Parse(spec); err != nil {
		return Blueprint{}, fmt.Errorf("%w: %s", domain.ErrValidation, err.Error())
	}
	version, err := s.store.NextVersion(ctx, orgID, current.Name)
	if err != nil {
		return Blueprint{}, err
	}
	return s.store.CreateVersion(ctx, orgID, current.Name, version, spec)
}

// Publish freezes version 1. A published spec can no longer change.
func (s *Service) Publish(ctx context.Context, orgID, id string) (Blueprint, error) {
	return s.store.Publish(ctx, orgID, id)
}

// Spec is the validated subset of the blueprint schema.
type Spec struct {
	SchemaVersion string `json:"schema_version"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Stack         struct {
		Frontend struct {
			Framework string `json:"framework"`
			Language  string `json:"language"`
		} `json:"frontend"`
		Backend struct {
			Language string `json:"language"`
		} `json:"backend"`
	} `json:"stack"`
	Repository struct {
		Layout string `json:"layout"`
	} `json:"repository"`
	TestStrategy json.RawMessage `json:"test_strategy"`
	Agents       json.RawMessage `json:"agents"`
	StarterTasks []StarterTask   `json:"starter_tasks"`
}

// StarterTask is one task created on instantiation (schema v2).
type StarterTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Acceptance  []struct {
		Given string `json:"given"`
		When  string `json:"when"`
		Then  string `json:"then"`
	} `json:"acceptance"`
}

// ValidationError lists schema violations.
type ValidationError struct{ Problems []string }

func (e *ValidationError) Error() string {
	return "blueprint: " + strings.Join(e.Problems, "; ")
}

// Parse validates a spec against the required blueprint rules.
func Parse(raw json.RawMessage) (Spec, error) {
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return Spec{}, &ValidationError{Problems: []string{"spec is not valid JSON"}}
	}

	var problems []string
	if spec.SchemaVersion != "1" && spec.SchemaVersion != "2" {
		problems = append(problems, "schema_version must be \"1\" or \"2\"")
	}
	if len(strings.TrimSpace(spec.Name)) < 3 || len(spec.Name) > 80 {
		problems = append(problems, "name must be 3-80 characters")
	}
	if !oneOf(spec.Stack.Backend.Language, "go", "node", "python", "java", "none") {
		problems = append(problems, "stack.backend.language is invalid")
	}
	if !oneOf(spec.Stack.Frontend.Framework, "next", "vite-react", "angular", "vue", "sveltekit", "none") {
		problems = append(problems, "stack.frontend.framework is invalid")
	}
	if !oneOf(spec.Stack.Frontend.Language, "typescript", "javascript") {
		problems = append(problems, "stack.frontend.language is invalid")
	}
	if !oneOf(spec.Repository.Layout, "monorepo", "polyrepo") {
		problems = append(problems, "repository.layout is invalid")
	}
	if len(spec.TestStrategy) == 0 {
		problems = append(problems, "test_strategy is required")
	}
	if len(spec.Agents) == 0 {
		problems = append(problems, "agents is required")
	}
	for i, task := range spec.StarterTasks {
		prefix := fmt.Sprintf("starter_tasks[%d].", i)
		if len(strings.TrimSpace(task.Title)) < 3 || len(task.Title) > 120 {
			problems = append(problems, prefix+"title must be 3-120 characters")
		}
		if len(task.Description) > 5000 {
			problems = append(problems, prefix+"description must be at most 5000 characters")
		}
		if task.Priority != "" && !oneOf(task.Priority, "p0", "p1", "p2") {
			problems = append(problems, prefix+"priority must be p0, p1 or p2")
		}
		for j, ac := range task.Acceptance {
			if strings.TrimSpace(ac.Given) == "" || strings.TrimSpace(ac.When) == "" || strings.TrimSpace(ac.Then) == "" {
				problems = append(problems, fmt.Sprintf("%sacceptance[%d] needs given, when and then", prefix, j))
			}
		}
	}
	if len(problems) > 0 {
		return Spec{}, &ValidationError{Problems: problems}
	}
	return spec, nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
