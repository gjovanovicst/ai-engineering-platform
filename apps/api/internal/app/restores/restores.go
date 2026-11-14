// Package restores queues PITR restore requests and exposes DR status.
package restores

import (
	"context"
	"fmt"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Request is a queued PITR restore.
type Request struct {
	ID         string
	TargetTime time.Time
	Status     string
	Detail     string
	CreatedAt  time.Time
}

// DR is the current backup/archive health.
type DR struct {
	WALFailed         int64
	WALArchived       int64
	WALAge            float64
	BaseAge           float64
	BaseName          string
	RestoreSucceeded  int64
	RestoreFailed     int64
	LastRestoreAge    float64
	AvgRestoreSeconds float64
	OffsiteAge        float64
}

// Store persists restore requests and reads DR metrics.
type Store interface {
	Create(ctx context.Context, orgID, actorID string, target time.Time) (Request, error)
	List(ctx context.Context, orgID string) ([]Request, error)
	DR(ctx context.Context, orgID string) (DR, error)
	Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error
}

// Service exposes restore use cases.
type Service struct{ store Store }

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

// Create queues a restore request.
func (s *Service) Create(ctx context.Context, orgID, actorID string, target time.Time) (Request, error) {
	target = target.UTC()
	if target.IsZero() {
		return Request{}, fmt.Errorf("%w: target_time is required", domain.ErrValidation)
	}
	if target.After(time.Now().Add(time.Minute)) {
		return Request{}, fmt.Errorf("%w: target_time must not be in the future", domain.ErrValidation)
	}
	req, err := s.store.Create(ctx, orgID, actorID, target)
	if err != nil {
		return Request{}, err
	}
	_ = s.store.Audit(ctx, orgID, actorID, "restore.requested", req.ID, []byte(`{}`))
	return req, nil
}

// List returns recent requests.
func (s *Service) List(ctx context.Context, orgID string) ([]Request, error) {
	return s.store.List(ctx, orgID)
}

// DR returns archive/base health.
func (s *Service) DR(ctx context.Context, orgID string) (DR, error) {
	return s.store.DR(ctx, orgID)
}
