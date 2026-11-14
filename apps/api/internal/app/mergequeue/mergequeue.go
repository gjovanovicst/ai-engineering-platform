// Package mergequeue serialises merges per project.
package mergequeue

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Item is one queued merge.
type Item struct {
	ID            int64
	OrgID         string
	ProjectID     string
	RunID         string
	TaskID        string
	PullRequestID string
	Number        int
	Status        string
	Error         string
	CreatedAt     time.Time
}

// Result is the outcome of processing one item.
type Result struct {
	Item   Item
	Merged bool
	Err    error
}

// Store persists the queue.
type Store interface {
	Enqueue(ctx context.Context, item Item) (int64, error)
	ClaimNext(ctx context.Context, orgID, projectID string) (Item, bool, error)
	Complete(ctx context.Context, orgID string, id int64, status, detail string) error
	Position(ctx context.Context, projectID string, id int64) (int, error)
	List(ctx context.Context, orgID, projectID string) ([]Item, error)
}

// Gate reports whether a PR is still blocked by required checks.
type Gate interface {
	Blocked(ctx context.Context, orgID, pullRequestID string) (bool, string, error)
}

// Merger applies the merge.
type Merger interface {
	Merge(ctx context.Context, orgID, projectID string, number int) error
}

// Rebaser is implemented by mergers that can update a branch first.
type Rebaser interface {
	Rebase(ctx context.Context, orgID, projectID string, number int) error
}

// Service enqueues and drains merges in order.
type Service struct {
	store  Store
	merger Merger
	gate   Gate
	log    *slog.Logger
}

// New wires the service.
func New(store Store, merger Merger, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, merger: merger, log: log}
}

// WithGate re-checks required checks immediately before merging.
func (s *Service) WithGate(gate Gate) *Service {
	s.gate = gate
	return s
}

// List returns recent queue entries for a project.
func (s *Service) List(ctx context.Context, orgID, projectID string) ([]Item, error) {
	return s.store.List(ctx, orgID, projectID)
}

// Enqueue adds a PR unless it is already queued/merging.
func (s *Service) Enqueue(ctx context.Context, item Item) (bool, error) {
	if item.ProjectID == "" || item.Number <= 0 {
		return false, errors.New("mergequeue: project and PR number are required")
	}
	id, err := s.store.Enqueue(ctx, item)
	if err != nil {
		return false, err
	}
	return id > 0, nil
}

// ProcessAll drains the queue for one project in id order. A failed merge is
// recorded and processing continues with the next item.
func (s *Service) ProcessAll(ctx context.Context, orgID, projectID string) ([]Result, error) {
	var results []Result
	for {
		item, ok, err := s.store.ClaimNext(ctx, orgID, projectID)
		if err != nil {
			return results, err
		}
		if !ok {
			return results, nil
		}
		if s.gate != nil && item.PullRequestID != "" {
			blocked, why, gerr := s.gate.Blocked(ctx, orgID, item.PullRequestID)
			if gerr != nil || blocked {
				detail := why
				if gerr != nil {
					detail = gerr.Error()
				}
				_ = s.store.Complete(ctx, orgID, item.ID, "failed", "checks: "+detail)
				results = append(results, Result{Item: item, Err: errors.New("checks: " + detail)})
				continue
			}
		}
		if rebaser, ok := s.merger.(Rebaser); ok {
			if rerr := rebaser.Rebase(ctx, orgID, projectID, item.Number); rerr != nil {
				_ = s.store.Complete(ctx, orgID, item.ID, "failed", "rebase: "+rerr.Error())
				results = append(results, Result{Item: item, Err: rerr})
				continue
			}
		}
		merr := s.merger.Merge(ctx, orgID, projectID, item.Number)
		status := "merged"
		detail := ""
		if merr != nil {
			status = "failed"
			detail = merr.Error()
			s.log.Warn("mergequeue: merge failed", slog.Int("pr", item.Number), slog.Any("err", merr))
		}
		if err := s.store.Complete(ctx, orgID, item.ID, status, detail); err != nil {
			return results, err
		}
		results = append(results, Result{Item: item, Merged: merr == nil, Err: merr})
	}
}
