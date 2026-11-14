package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/deployments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// DeploymentStore implements deployments.Store.
type DeploymentStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewDeploymentStore wires the store.
func NewDeploymentStore(pool *pgxpool.Pool) *DeploymentStore {
	return &DeploymentStore{pool: pool, q: pgstore.New(pool)}
}

// CreateEnvironment inserts an environment.
func (s *DeploymentStore) CreateEnvironment(ctx context.Context, e deployments.Environment) (deployments.Environment, error) {
	var out deployments.Environment
	err := db.WithOrg(ctx, s.pool, e.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateEnvironment(ctx, pgstore.CreateEnvironmentParams{
			OrgID: toUUID(e.OrgID), ProjectID: toUUID(e.ProjectID),
			Name: pgstore.EnvName(e.Name), Url: nilIfEmpty(e.URL),
		})
		if err != nil {
			return mapUnique(err)
		}
		out = environmentFromRow(row)
		return nil
	})
	return out, err
}

// ListEnvironments lists project environments.
func (s *DeploymentStore) ListEnvironments(ctx context.Context, orgID, projectID string) ([]deployments.Environment, error) {
	var out []deployments.Environment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListEnvironments(ctx, pgstore.ListEnvironmentsParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			return err
		}
		out = make([]deployments.Environment, 0, len(rows))
		for _, row := range rows {
			out = append(out, environmentFromRow(row))
		}
		return nil
	})
	return out, err
}

// GetEnvironment returns one environment.
func (s *DeploymentStore) GetEnvironment(ctx context.Context, orgID, id string) (deployments.Environment, error) {
	var out deployments.Environment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetEnvironment(ctx, pgstore.GetEnvironmentParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = environmentFromRow(row)
		return nil
	})
	return out, err
}

// CreateDeployment inserts a deployment.
func (s *DeploymentStore) CreateDeployment(ctx context.Context, d deployments.Deployment) (deployments.Deployment, error) {
	var out deployments.Deployment
	err := db.WithOrg(ctx, s.pool, d.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateDeployment(ctx, pgstore.CreateDeploymentParams{
			OrgID: toUUID(d.OrgID), EnvironmentID: toUUID(d.EnvironmentID),
			ReleaseRef: d.ReleaseRef, Status: pgstore.DeployStatus(d.Status),
			DeployedBy: optionalUUID(d.DeployedBy), StartedAt: timePtr(d.StartedAt),
			FinishedAt: timePtr(d.FinishedAt), Log: d.Log,
		})
		if err != nil {
			return err
		}
		out = deploymentFromRow(row)
		out.Environment = d.Environment
		return nil
	})
	return out, err
}

// ListDeployments lists deployments of an environment.
func (s *DeploymentStore) ListDeployments(ctx context.Context, orgID, environmentID string, limit int) ([]deployments.Deployment, error) {
	var out []deployments.Deployment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListDeployments(ctx, pgstore.ListDeploymentsParams{
			OrgID: toUUID(orgID), EnvironmentID: toUUID(environmentID),
			Limit: int32(limit), //nolint:gosec // clamped
		})
		if err != nil {
			return err
		}
		out = make([]deployments.Deployment, 0, len(rows))
		for _, row := range rows {
			out = append(out, deploymentFromRow(row))
		}
		return nil
	})
	return out, err
}

// GetDeployment returns one deployment.
func (s *DeploymentStore) GetDeployment(ctx context.Context, orgID, id string) (deployments.Deployment, error) {
	var out deployments.Deployment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetDeployment(ctx, pgstore.GetDeploymentParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = deploymentFromRow(row)
		return nil
	})
	return out, err
}

// ApproveDeployment approves a queued deployment.
func (s *DeploymentStore) ApproveDeployment(ctx context.Context, orgID, id, userID string) (deployments.Deployment, error) {
	var out deployments.Deployment
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ApproveDeployment(ctx, pgstore.ApproveDeploymentParams{
			ID: toUUID(id), OrgID: toUUID(orgID), ApprovedBy: toUUID(userID), Log: userID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTransition
			}
			return err
		}
		out = deploymentFromRow(row)
		return nil
	})
	return out, err
}

func environmentFromRow(row pgstore.Environment) deployments.Environment {
	e := deployments.Environment{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
		Name: string(row.Name), CreatedAt: row.CreatedAt.Time,
	}
	if row.Url != nil {
		e.URL = *row.Url
	}
	return e
}

func deploymentFromRow(row pgstore.Deployment) deployments.Deployment {
	d := deployments.Deployment{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), EnvironmentID: uuidString(row.EnvironmentID),
		ReleaseRef: row.ReleaseRef, Status: string(row.Status), Log: row.Log,
	}
	if row.DeployedBy.Valid {
		d.DeployedBy = uuidString(row.DeployedBy)
	}
	if row.ApprovedBy.Valid {
		d.ApprovedBy = uuidString(row.ApprovedBy)
	}
	if row.StartedAt.Valid {
		v := row.StartedAt.Time
		d.StartedAt = &v
	}
	if row.FinishedAt.Valid {
		v := row.FinishedAt.Time
		d.FinishedAt = &v
	}
	return d
}
