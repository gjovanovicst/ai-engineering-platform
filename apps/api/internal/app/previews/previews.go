// Package previews manages ephemeral per-PR environments.
package previews

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Environment is one preview.
type Environment struct {
	ID          string
	OrgID       string
	ProjectID   string
	Name        string
	Image       string
	Port        int
	Status      string
	URL         string
	ContainerID string
	Error       string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// Store persists previews.
type Store interface {
	Create(ctx context.Context, env Environment) (Environment, error)
	List(ctx context.Context, orgID, projectID string) ([]Environment, error)
	Get(ctx context.Context, orgID, id string) (Environment, error)
	SetStatus(ctx context.Context, orgID, id, status, url, containerID, detail string) error
}

// Driver starts and stops containers.
type Driver interface {
	Start(ctx context.Context, image string, port int) (url, containerID string, err error)
	Stop(ctx context.Context, containerID string) error
}

// Scheduler defers start/stop to a worker.
type Scheduler interface {
	Schedule(ctx context.Context, previewID, orgID, action string, after time.Duration) error
}

// Service creates and tears down previews.
type Service struct {
	store     Store
	driver    Driver
	scheduler Scheduler
}

// New wires the service; the driver is required only to start/stop.
func New(store Store, driver Driver) (*Service, error) {
	if store == nil {
		return nil, errors.New("previews: store is required")
	}
	return &Service{store: store, driver: driver}, nil
}

// WithScheduler defers container lifecycle to the worker.
func (s *Service) WithScheduler(scheduler Scheduler) *Service {
	s.scheduler = scheduler
	return s
}

// Create validates and records a requested preview (TTL 5 min .. 24 h).
func (s *Service) Create(ctx context.Context, orgID, projectID, name, image string, port int, ttl time.Duration) (Environment, error) {
	image = strings.TrimSpace(image)
	if image == "" || strings.ContainsAny(image, " \t\n") {
		return Environment{}, fmt.Errorf("%w: a valid image is required", domain.ErrValidation)
	}
	if port <= 0 || port > 65535 {
		return Environment{}, fmt.Errorf("%w: port must be 1..65535", domain.ErrValidation)
	}
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	if ttl < 5*time.Minute || ttl > 24*time.Hour {
		return Environment{}, fmt.Errorf("%w: TTL must be between 5 minutes and 24 hours", domain.ErrValidation)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "preview"
	}
	env, err := s.store.Create(ctx, Environment{
		OrgID: orgID, ProjectID: projectID, Name: name, Image: image, Port: port,
		Status: "requested", ExpiresAt: time.Now().UTC().Add(ttl),
	})
	if err != nil {
		return Environment{}, err
	}
	if s.scheduler != nil {
		_ = s.scheduler.Schedule(ctx, env.ID, orgID, "start", 0)
		_ = s.scheduler.Schedule(ctx, env.ID, orgID, "stop", ttl)
	}
	return env, nil
}

// List returns recent previews for a project.
func (s *Service) List(ctx context.Context, orgID, projectID string) ([]Environment, error) {
	return s.store.List(ctx, orgID, projectID)
}

// Start launches a requested preview.
func (s *Service) Start(ctx context.Context, orgID, id string) error {
	env, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if env.Status != "requested" && env.Status != "failed" {
		return nil
	}
	if time.Now().After(env.ExpiresAt) {
		return s.store.SetStatus(ctx, orgID, id, "destroyed", "", "", "expired before start")
	}
	if s.driver == nil {
		return fmt.Errorf("previews: driver is not configured")
	}
	if err := s.store.SetStatus(ctx, orgID, id, "starting", "", "", ""); err != nil {
		return err
	}
	rawURL, containerID, derr := s.driver.Start(ctx, env.Image, env.Port)
	if derr != nil {
		_ = s.store.SetStatus(ctx, orgID, id, "failed", "", "", derr.Error())
		return derr
	}
	return s.store.SetStatus(ctx, orgID, id, "running", rawURL, containerID, "")
}

// RequestDestroy asks the worker to stop a preview.
func (s *Service) RequestDestroy(ctx context.Context, orgID, id string) error {
	env, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if env.Status == "destroyed" {
		return nil
	}
	if err := s.store.SetStatus(ctx, orgID, id, "destroying", env.URL, env.ContainerID, ""); err != nil {
		return err
	}
	if s.scheduler != nil {
		return s.scheduler.Schedule(ctx, id, orgID, "stop", 0)
	}
	return nil
}

// Destroy stops a preview (idempotent).
func (s *Service) Destroy(ctx context.Context, orgID, id string) error {
	env, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if env.Status == "destroyed" {
		return nil
	}
	_ = s.store.SetStatus(ctx, orgID, id, "destroying", env.URL, env.ContainerID, "")
	if env.ContainerID != "" && s.driver == nil {
		return fmt.Errorf("previews: driver is not configured")
	}
	if env.ContainerID != "" {
		if derr := s.driver.Stop(ctx, env.ContainerID); derr != nil {
			_ = s.store.SetStatus(ctx, orgID, id, "failed", env.URL, env.ContainerID, derr.Error())
			return derr
		}
	}
	return s.store.SetStatus(ctx, orgID, id, "destroyed", "", "", "")
}

// ValidURL is a small helper for tests/consumers.
func ValidURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && u.Host != ""
}
