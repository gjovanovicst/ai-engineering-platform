// Package search finds tasks across a workspace.
package search

import (
	"context"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Store runs the query.
type Store interface {
	SearchTasks(ctx context.Context, orgID, projectID, query string, limit int) ([]domain.Task, error)
}

// Service validates and runs searches.
type Service struct {
	store Store
}

// New wires the service.
func New(store Store) *Service { return &Service{store: store} }

// Search returns tasks matching the query. An empty query returns nothing:
// listing is the project endpoint's job.
func (s *Service) Search(ctx context.Context, orgID, projectID, query string, limit int) ([]domain.Task, error) {
	q := strings.TrimSpace(query)
	if len(q) < 2 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.store.SearchTasks(ctx, orgID, projectID, q, limit)
}
