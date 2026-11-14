// Package deployments records environment releases and the prod four-eyes gate.
package deployments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Environment is dev/staging/prod for a project.
type Environment struct {
	ID        string
	OrgID     string
	ProjectID string
	Name      string
	URL       string
	CreatedAt time.Time
}

// Deployment is one release attempt.
type Deployment struct {
	ID            string
	OrgID         string
	EnvironmentID string
	Environment   string
	ReleaseRef    string
	Status        string
	DeployedBy    string
	ApprovedBy    string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	Log           string
}

// Store persists environments and deployments.
type Store interface {
	CreateEnvironment(ctx context.Context, e Environment) (Environment, error)
	ListEnvironments(ctx context.Context, orgID, projectID string) ([]Environment, error)
	GetEnvironment(ctx context.Context, orgID, id string) (Environment, error)
	CreateDeployment(ctx context.Context, d Deployment) (Deployment, error)
	ListDeployments(ctx context.Context, orgID, environmentID string, limit int) ([]Deployment, error)
	GetDeployment(ctx context.Context, orgID, id string) (Deployment, error)
	ApproveDeployment(ctx context.Context, orgID, id, userID string) (Deployment, error)
}

// Service validates and records deployments.
type Service struct {
	store Store
	log   *slog.Logger
}

// New wires the service.
func New(store Store, log *slog.Logger) (*Service, error) {
	if store == nil {
		return nil, errors.New("deployments: store is required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, log: log}, nil
}

// CreateEnvironment adds dev/staging/prod to a project.
func (s *Service) CreateEnvironment(ctx context.Context, orgID, projectID, name, url string) (Environment, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "dev" && name != "staging" && name != "prod" {
		return Environment{}, fmt.Errorf("%w: environment must be dev, staging or prod", domain.ErrValidation)
	}
	return s.store.CreateEnvironment(ctx, Environment{
		OrgID: orgID, ProjectID: projectID, Name: name, URL: strings.TrimSpace(url),
	})
}

// ListEnvironments returns the project environments.
func (s *Service) ListEnvironments(ctx context.Context, orgID, projectID string) ([]Environment, error) {
	return s.store.ListEnvironments(ctx, orgID, projectID)
}

// Deploy records a release. Prod deployments wait for a second admin.
func (s *Service) Deploy(ctx context.Context, orgID, environmentID, releaseRef, userID string) (Deployment, error) {
	release := strings.TrimSpace(releaseRef)
	if release == "" {
		return Deployment{}, fmt.Errorf("%w: release_ref is required", domain.ErrValidation)
	}
	env, err := s.store.GetEnvironment(ctx, orgID, environmentID)
	if err != nil {
		return Deployment{}, err
	}
	now := time.Now().UTC()
	deployment := Deployment{
		OrgID: orgID, EnvironmentID: env.ID, Environment: env.Name,
		ReleaseRef: release, DeployedBy: userID,
	}
	if env.Name == "prod" {
		deployment.Status = "queued"
		deployment.Log = "waiting for a second admin approval\n"
	} else {
		deployment.Status = "succeeded"
		deployment.StartedAt = &now
		deployment.FinishedAt = &now
		deployment.Log = "deployed to " + env.Name + "\n"
	}
	return s.store.CreateDeployment(ctx, deployment)
}

// ListDeployments returns the environment deployments.
func (s *Service) ListDeployments(ctx context.Context, orgID, environmentID string, limit int) ([]Deployment, error) {
	return s.store.ListDeployments(ctx, orgID, environmentID, limit)
}

// Approve releases a queued prod deployment. The approver must differ from the
// requester (four eyes) and the handler enforces the admin role.
func (s *Service) Approve(ctx context.Context, orgID, deploymentID, approverID string) (Deployment, error) {
	deployment, err := s.store.GetDeployment(ctx, orgID, deploymentID)
	if err != nil {
		return Deployment{}, err
	}
	if deployment.Status != "queued" {
		return Deployment{}, &domain.TransitionError{
			From: domain.TaskStatus(deployment.Status), To: "succeeded",
			Detail: "only queued deployments can be approved",
		}
	}
	if deployment.DeployedBy == approverID {
		return Deployment{}, fmt.Errorf("%w: the approver must be a different admin", domain.ErrValidation)
	}
	return s.store.ApproveDeployment(ctx, orgID, deploymentID, approverID)
}

// Logs returns the deployment log.
func (s *Service) Logs(ctx context.Context, orgID, deploymentID string) (string, error) {
	deployment, err := s.store.GetDeployment(ctx, orgID, deploymentID)
	if err != nil {
		return "", err
	}
	return deployment.Log, nil
}
