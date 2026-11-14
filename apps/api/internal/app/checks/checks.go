// Package checks stores CI check results reported by git providers.
package checks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Check is one CI check result for a pull request.
type Check struct {
	ID         string
	Name       string
	Status     string // pending, running, passed, failed
	Output     string
	DurationMS int
	CommitSHA  string
}

// Store persists check results.
type Store interface {
	Upsert(ctx context.Context, orgID, repoID, prID, runID string, c Check) (Check, error)
	ListForPullRequest(ctx context.Context, orgID, prID string) ([]Check, error)
	ListForRun(ctx context.Context, orgID, runID string) ([]Check, error)
	Failed(ctx context.Context, orgID, prID string) (bool, error)
	InsertForRun(ctx context.Context, orgID, runID string, c Check) (Check, error)
	RequiredForPullRequest(ctx context.Context, orgID, prID string) ([]string, error)
}

// Service validates check writes.
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

var statuses = map[string]bool{"pending": true, "running": true, "passed": true, "failed": true}

// Record stores or updates one check.
func (s *Service) Record(ctx context.Context, orgID, repoID, prID, runID string, c Check) (Check, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return Check{}, fmt.Errorf("%w: check name is required", domain.ErrValidation)
	}
	if c.Status == "" {
		c.Status = "pending"
	}
	if !statuses[c.Status] {
		return Check{}, fmt.Errorf("%w: unknown check status %q", domain.ErrValidation, c.Status)
	}
	return s.store.Upsert(ctx, orgID, repoID, prID, runID, c)
}

// RecordRunTest stores the test phase result of a run.
func (s *Service) RecordRunTest(ctx context.Context, orgID, runID, name, status, output string) error {
	if name == "" {
		name = "test"
	}
	if status != "passed" && status != "failed" && status != "running" {
		return fmt.Errorf("%w: unknown test status", domain.ErrValidation)
	}
	_, err := s.store.InsertForRun(ctx, orgID, runID, Check{Name: name, Status: status, Output: output})
	return err
}

// ListForPullRequest returns the checks of a PR.
func (s *Service) ListForPullRequest(ctx context.Context, orgID, prID string) ([]Check, error) {
	return s.store.ListForPullRequest(ctx, orgID, prID)
}

// ListForRun returns the checks recorded for a run.
func (s *Service) ListForRun(ctx context.Context, orgID, runID string) ([]Check, error) {
	return s.store.ListForRun(ctx, orgID, runID)
}

// Blocked is the merge gate: it reports whether a failed check or an unmet
// required check forbids the merge, with a human-readable reason.
func (s *Service) Blocked(ctx context.Context, orgID, prID string) (bool, string, error) {
	items, err := s.store.ListForPullRequest(ctx, orgID, prID)
	if err != nil {
		return false, "", err
	}
	byName := make(map[string]Check, len(items))
	for _, item := range items {
		byName[item.Name] = item
		if item.Status == "failed" {
			return true, "check " + item.Name + " failed", nil
		}
	}
	required, err := s.store.RequiredForPullRequest(ctx, orgID, prID)
	if err != nil {
		return false, "", err
	}
	for _, name := range required {
		item, ok := byName[name]
		if !ok {
			return true, "required check " + name + " has not run", nil
		}
		if item.Status != "passed" {
			return true, "required check " + name + " is " + item.Status, nil
		}
	}
	return false, "", nil
}

// Failed reports whether any check of the PR failed (merge gate).
func (s *Service) Failed(ctx context.Context, orgID, prID string) (bool, error) {
	return s.store.Failed(ctx, orgID, prID)
}
