// Package tasks implements task use cases on top of the domain state machine.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Store is the persistence port.
type Store interface {
	Create(ctx context.Context, t domain.Task) (domain.Task, error)
	Get(ctx context.Context, orgID, id string) (domain.Task, error)
	GetByRef(ctx context.Context, orgID, projectID, ref string) (domain.Task, error)
	NextTaskRef(ctx context.Context, orgID, projectID string) (int64, error)
	List(ctx context.Context, orgID, projectID, status, assignee string, limit int) ([]domain.Task, error)
	Count(ctx context.Context, orgID, projectID string) (int, error)
	UpdateStatus(ctx context.Context, t domain.Task) (domain.Task, error)
	Update(ctx context.Context, t domain.Task) (domain.Task, error)
}

// Service holds the task use cases.
type Service struct{ store Store }

// New returns a task service.
func New(store Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("tasks: store is required")
	}
	return &Service{store: store}, nil
}

// CreateInput is the create-task payload.
type CreateInput struct {
	OrgID       string
	ProjectID   string
	Title       string
	Description string
	Acceptance  []domain.AcceptanceCriterion
	Priority    domain.Priority
	Tags        []string
	CreatedBy   string
	DependsOn   []string
	Files       []string
	OutOfScope  []string
	TestPlan    []string
	Rollback    []string
}

// Create assigns the next reference and stores the task in draft.
func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Task, error) {
	priority := in.Priority
	if priority == "" {
		priority = domain.PriorityMed
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		seq, err := s.store.NextTaskRef(ctx, in.OrgID, in.ProjectID)
		if err != nil {
			return domain.Task{}, err
		}
		ref := "T-" + strconv.FormatInt(seq, 10)
		task, err := domain.NewTask(in.OrgID, in.ProjectID, ref, in.Title)
		if err != nil {
			return domain.Task{}, err
		}
		task.Description = in.Description
		task.Acceptance = in.Acceptance
		task.Priority = priority
		task.Tags = in.Tags
		task.Files = in.Files
		task.OutOfScope = in.OutOfScope
		task.TestPlan = in.TestPlan
		task.Rollback = in.Rollback
		task.CreatedBy = in.CreatedBy
		for _, ref := range in.DependsOn {
			dep, derr := s.store.GetByRef(ctx, in.OrgID, in.ProjectID, ref)
			if derr != nil {
				return domain.Task{}, fmt.Errorf("%w: dependency %s not found", domain.ErrValidation, ref)
			}
			seen := false
			for _, existing := range task.DependsOn {
				if existing == dep.ID {
					seen = true
				}
			}
			if !seen {
				task.DependsOn = append(task.DependsOn, dep.ID)
			}
		}
		if len(task.DependsOn) > 20 {
			return domain.Task{}, fmt.Errorf("%w: at most 20 dependencies", domain.ErrValidation)
		}
		if err := task.Validate(); err != nil {
			return domain.Task{}, err
		}

		created, err := s.store.Create(ctx, *task)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, domain.ErrConflict) {
			return domain.Task{}, err
		}
		lastErr = err // reference collision: try the next number
	}
	return domain.Task{}, fmt.Errorf("tasks: could not allocate a reference: %w", lastErr)
}

// Get resolves a task by UUID or by its reference (T-88).
func (s *Service) Get(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error) {
	t, err := s.store.Get(ctx, orgID, idOrRef)
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Task{}, err
	}
	return s.store.GetByRef(ctx, orgID, projectID, idOrRef)
}

// List returns tasks of a project with optional filters.
func (s *Service) List(ctx context.Context, orgID, projectID, status, assignee string, limit int) ([]domain.Task, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if status != "" {
		if _, err := domain.ParseTaskStatus(status); err != nil {
			return nil, err
		}
	}
	return s.store.List(ctx, orgID, projectID, status, assignee, limit)
}

// MarkMerged moves a reviewed task to done when its PR/MR is merged.
func (s *Service) MarkMerged(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error) {
	task, err := s.Get(ctx, orgID, projectID, idOrRef)
	if err != nil {
		return domain.Task{}, err
	}
	// A merged PR can arrive while the task is still in_progress (e.g. the
	// review move did not run); walk it through review, then done.
	if task.Status == domain.TaskInProgress {
		if err := task.Transition(domain.TaskReview, domain.TransitionOptions{
			Actor: domain.ActorSystem, TestsGreen: true, Reason: "pull request merged",
		}); err != nil {
			return domain.Task{}, err
		}
	}
	if err := task.Transition(domain.TaskDone, domain.TransitionOptions{
		Actor: domain.ActorSystem, PRMerged: true, Reason: "pull request merged",
	}); err != nil {
		return domain.Task{}, err
	}
	return s.store.UpdateStatus(ctx, task)
}

// UpdateStatus persists a task whose state machine already moved.
func (s *Service) UpdateStatus(ctx context.Context, t domain.Task) (domain.Task, error) {
	return s.store.UpdateStatus(ctx, t)
}

// UpdateInput edits the mutable fields of a task.
type UpdateInput struct {
	OrgID       string
	ProjectID   string
	IDOrRef     string
	Title       string
	Description string
	Priority    domain.Priority
	AssigneeID  string
	Tags        []string
	Acceptance  []domain.AcceptanceCriterion
	DependsOn   []string
	Files       []string
	OutOfScope  []string
	TestPlan    []string
	Rollback    []string
}

// Update edits a task (status changes go through Transition).
func (s *Service) Update(ctx context.Context, in UpdateInput) (domain.Task, error) {
	task, err := s.Get(ctx, in.OrgID, in.ProjectID, in.IDOrRef)
	if err != nil {
		return domain.Task{}, err
	}
	if err := task.Validate(); err != nil {
		return domain.Task{}, err
	}
	task.Title = in.Title
	task.Description = in.Description
	if in.Priority != "" {
		task.Priority = in.Priority
	}
	task.AssigneeID = in.AssigneeID
	if in.Tags != nil {
		task.Tags = in.Tags
	}
	if in.Acceptance != nil {
		task.Acceptance = in.Acceptance
	}
	if in.Files != nil {
		task.Files = in.Files
	}
	if in.OutOfScope != nil {
		task.OutOfScope = in.OutOfScope
	}
	if in.TestPlan != nil {
		task.TestPlan = in.TestPlan
	}
	if in.Rollback != nil {
		task.Rollback = in.Rollback
	}
	if in.DependsOn != nil {
		task.DependsOn = nil
		for _, ref := range in.DependsOn {
			dep, derr := s.store.GetByRef(ctx, in.OrgID, in.ProjectID, ref)
			if derr != nil {
				return domain.Task{}, fmt.Errorf("%w: dependency %s not found", domain.ErrValidation, ref)
			}
			if dep.ID == task.ID {
				return domain.Task{}, fmt.Errorf("%w: a task cannot depend on itself", domain.ErrValidation)
			}
			seen := false
			for _, existing := range task.DependsOn {
				if existing == dep.ID {
					seen = true
				}
			}
			if !seen {
				task.DependsOn = append(task.DependsOn, dep.ID)
			}
		}
		if len(task.DependsOn) > 20 {
			return domain.Task{}, fmt.Errorf("%w: at most 20 dependencies", domain.ErrValidation)
		}
	}
	if err := task.Validate(); err != nil {
		return domain.Task{}, err
	}
	return s.store.Update(ctx, task)
}

// Transition applies the state machine as a human action.
func (s *Service) Transition(ctx context.Context, orgID, idOrRef, projectID, to, reason string) (domain.Task, error) {
	task, err := s.Get(ctx, orgID, projectID, idOrRef)
	if err != nil {
		return domain.Task{}, err
	}
	status, err := domain.ParseTaskStatus(to)
	if err != nil {
		return domain.Task{}, err
	}
	if err := task.Transition(status, domain.TransitionOptions{
		Actor:  domain.ActorHuman,
		Reason: reason,
	}); err != nil {
		return domain.Task{}, err
	}
	return s.store.UpdateStatus(ctx, task)
}

// DependenciesBlocked returns a dependency ref that is not done yet.
func (s *Service) DependenciesBlocked(ctx context.Context, orgID, projectID, taskID string) (string, error) {
	task, err := s.store.Get(ctx, orgID, taskID)
	if err != nil {
		return "", err
	}
	for _, id := range task.DependsOn {
		dep, derr := s.store.Get(ctx, orgID, id)
		if derr != nil {
			return "", derr
		}
		if dep.Status != domain.TaskDone {
			return dep.Ref, nil
		}
	}
	return "", nil
}
