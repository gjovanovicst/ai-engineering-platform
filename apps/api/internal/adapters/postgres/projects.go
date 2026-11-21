package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// ProjectStore implements projects.Store on PostgreSQL. Every query runs with
// the org context so the RLS policy applies.
type ProjectStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewProjectStore wires the store to a pool.
func NewProjectStore(pool *pgxpool.Pool) *ProjectStore {
	return &ProjectStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts a project for its org.
func (s *ProjectStore) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, p.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateProject(ctx, pgstore.CreateProjectParams{
			OrgID:       toUUID(p.OrgID),
			Name:        p.Name,
			Slug:        p.Slug,
			Description: p.Description, BlueprintID: toUUID(p.BlueprintID),
		})
		if err != nil {
			return mapUnique(err)
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// Get returns one project of the org.
func (s *ProjectStore) Get(ctx context.Context, orgID, id string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetProject(ctx, pgstore.GetProjectParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// List returns projects of the org, newest first.
func (s *ProjectStore) List(ctx context.Context, orgID string, limit int) ([]domain.Project, error) {
	var out []domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListProjects(ctx, pgstore.ListProjectsParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // limit is clamped to 100 by the service
		})
		if err != nil {
			return err
		}
		out = make([]domain.Project, 0, len(rows))
		for _, row := range rows {
			out = append(out, projectFromRow(row))
		}
		return nil
	})
	return out, err
}

// UpdateRequiredChecks updates the merge-gate check list.
func (s *ProjectStore) UpdateRequiredChecks(ctx context.Context, orgID, id string, checks []string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateProjectRequiredChecks(ctx, pgstore.UpdateProjectRequiredChecksParams{
			ID: toUUID(id), OrgID: toUUID(orgID), RequiredChecks: checks,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

func projectFromRow(row pgstore.Project) domain.Project {
	return domain.Project{
		ID:                    uuidString(row.ID),
		OrgID:                 uuidString(row.OrgID),
		Name:                  row.Name,
		Slug:                  row.Slug,
		Description:           row.Description,
		AgentPluginID:         uuidPtr(row.AgentPluginID),
		DesignStandardID:      uuidString(row.DesignSystemID),
		DesignStandardVersion: intFromPtr(row.DesignStandardVersion),
		RequiredChecks:        row.RequiredChecks,
		E2ECommand:            row.E2eCommand, TestCommand: row.TestCommand, VisualCommand: row.VisualCommand, MCPServers: row.McpServers,
		VisualThreshold: row.VisualThreshold, VisualMasks: row.VisualMasks,
		PolicyMaxDiffBytes: int(row.PolicyMaxDiffBytes), PolicyForbiddenPaths: row.PolicyForbiddenPaths, PolicyChangeWindow: row.PolicyChangeWindow, PolicyAllowedModels: row.PolicyAllowedModels, PolicyAllowedTools: row.PolicyAllowedTools, PolicyRequiredApprovals: int(row.PolicyRequiredApprovals), PluginChecks: row.PluginChecks,
		Status:    domain.ProjectStatus(row.Status),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

var _ interface {
	Create(context.Context, domain.Project) (domain.Project, error)
	Get(context.Context, string, string) (domain.Project, error)
	List(context.Context, string, int) ([]domain.Project, error)
} = (*ProjectStore)(nil)

// SetDesignStandard assigns a published standard or clears it with an empty id.
func (s *ProjectStore) SetDesignStandard(ctx context.Context, orgID, projectID, standardID string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if standardID == "" {
			row, err := q.ClearProjectDesignStandard(ctx, pgstore.ClearProjectDesignStandardParams{
				ID: toUUID(projectID), OrgID: toUUID(orgID),
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrNotFound
				}
				return err
			}
			out = projectFromRow(row)
			return nil
		}
		row, err := q.SetProjectDesignStandard(ctx, pgstore.SetProjectDesignStandardParams{
			ID: toUUID(projectID), OrgID: toUUID(orgID), ID_2: toUUID(standardID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: the design standard must be published", domain.ErrValidation)
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// DesignStandard returns the standard assigned to a project.
func (s *ProjectStore) DesignStandard(ctx context.Context, orgID, projectID string) (projects.ProjectStandard, error) {
	var out projects.ProjectStandard
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetProjectDesignStandard(ctx, pgstore.GetProjectDesignStandardParams{
			ID: toUUID(projectID), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projects.ProjectStandard{ID: uuidString(row.ID), Name: row.Name, Version: int(row.Version), SHA256: hex.EncodeToString(row.Sha256)}
		if row.PublishedAt.Valid {
			out.PublishedAt = &row.PublishedAt.Time
		}
		return nil
	})
	return out, err
}

func intFromPtr(v *int32) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

// ListComponents returns the project's allowlist.
func (s *ProjectStore) ListComponents(ctx context.Context, orgID, projectID string) ([]projects.ProjectComponent, error) {
	var out []projects.ProjectComponent
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListProjectComponents(ctx, pgstore.ListProjectComponentsParams{
			ProjectID: toUUID(projectID), OrgID: toUUID(orgID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, projects.ProjectComponent{
				ID: uuidString(row.ID), Name: row.Name, Category: row.Category, Status: row.Status,
				ImportPath: row.ImportPath, Package: row.Package, Version: row.Version,
				Mode: row.Mode, Reason: row.Reason,
			})
		}
		return nil
	})
	return out, err
}

// ReplaceComponents replaces the allowlist in one transaction.
func (s *ProjectStore) ReplaceComponents(ctx context.Context, orgID, projectID, actorID string, selections []projects.ComponentSelection) (int, error) {
	inserted := 0
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.DeleteProjectComponents(ctx, pgstore.DeleteProjectComponentsParams{
			ProjectID: toUUID(projectID), OrgID: toUUID(orgID),
		}); err != nil {
			return err
		}
		for _, selection := range selections {
			n, err := q.InsertProjectComponent(ctx, pgstore.InsertProjectComponentParams{
				OrgID: toUUID(orgID), ProjectID: toUUID(projectID), ID: toUUID(selection.ID),
				CreatedBy: toUUID(actorID), Mode: selection.Mode, Reason: selection.Reason,
			})
			if err != nil {
				return err
			}
			inserted += int(n)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

// SetE2ECommand stores the project e2e command.
func (s *ProjectStore) SetE2ECommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectE2ECommand(ctx, pgstore.SetProjectE2ECommandParams{
			ID: toUUID(id), OrgID: toUUID(orgID), E2eCommand: command,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// SetTestCommand stores the project test command.
func (s *ProjectStore) SetTestCommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectTestCommand(ctx, pgstore.SetProjectTestCommandParams{
			ID: toUUID(id), OrgID: toUUID(orgID), TestCommand: command,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// Update changes the project's name and description (nil keeps a field).
func (s *ProjectStore) Update(ctx context.Context, orgID, id string, name, description *string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpdateProject(ctx, pgstore.UpdateProjectParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Name: name, Description: description,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// TestCommand returns the project's test command (empty means the global one).
func (s *ProjectStore) TestCommand(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.TestCommand, nil
}

// E2ECommand returns the project's e2e command.
func (s *ProjectStore) E2ECommand(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.E2ECommand, nil
}

// VisualCommand returns the project's visual diff command.
func (s *ProjectStore) VisualCommand(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.VisualCommand, nil
}

// SetVisualCommand stores the project visual command.
func (s *ProjectStore) SetVisualCommand(ctx context.Context, orgID, id, command string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectVisualCommand(ctx, pgstore.SetProjectVisualCommandParams{
			ID: toUUID(id), OrgID: toUUID(orgID), VisualCommand: command,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// SetMCPConfig stores the project MCP server JSON.
func (s *ProjectStore) SetMCPConfig(ctx context.Context, orgID, id, config string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectMCP(ctx, pgstore.SetProjectMCPParams{
			ID: toUUID(id), OrgID: toUUID(orgID), McpServers: config,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// MCPConfig returns the project MCP server JSON.
func (s *ProjectStore) MCPConfig(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.MCPServers, nil
}

// SetVisualPolicy stores the per-project screenshot policy.
func (s *ProjectStore) SetVisualPolicy(ctx context.Context, orgID, id string, threshold float64, masks string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectVisualPolicy(ctx, pgstore.SetProjectVisualPolicyParams{
			ID: toUUID(id), OrgID: toUUID(orgID), VisualThreshold: threshold, VisualMasks: masks,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// VisualPolicy returns the project screenshot threshold and masks JSON.
func (s *ProjectStore) VisualPolicy(ctx context.Context, orgID, projectID string) (float64, string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return 0, "", err
	}
	return project.VisualThreshold, project.VisualMasks, nil
}

// SetPolicy stores the diff-size limit and forbidden paths.
func (s *ProjectStore) SetPolicy(ctx context.Context, orgID, id string, maxBytes int, paths []string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectPolicy(ctx, pgstore.SetProjectPolicyParams{
			ID: toUUID(id), OrgID: toUUID(orgID), PolicyMaxDiffBytes: int32(maxBytes), //nolint:gosec // validated 0..5 MiB
			Column4: paths,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// Policy returns the diff-size limit and forbidden paths.
func (s *ProjectStore) Policy(ctx context.Context, orgID, projectID string) (int, []string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return 0, nil, err
	}
	return project.PolicyMaxDiffBytes, project.PolicyForbiddenPaths, nil
}

// SetChangeWindow stores the project merge window JSON.
func (s *ProjectStore) SetChangeWindow(ctx context.Context, orgID, id, window string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectChangeWindow(ctx, pgstore.SetProjectChangeWindowParams{
			ID: toUUID(id), OrgID: toUUID(orgID), PolicyChangeWindow: window,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// SetAllowedModels stores the project model allowlist.
func (s *ProjectStore) SetAllowedModels(ctx context.Context, orgID, id string, models []string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectAllowedModels(ctx, pgstore.SetProjectAllowedModelsParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Column3: models,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// AllowedModels returns the project model allowlist.
func (s *ProjectStore) AllowedModels(ctx context.Context, orgID, projectID string) ([]string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	return project.PolicyAllowedModels, nil
}

// SetAllowedTools stores the MCP tool allowlist.
func (s *ProjectStore) SetAllowedTools(ctx context.Context, orgID, id string, tools []string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectAllowedTools(ctx, pgstore.SetProjectAllowedToolsParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Column3: tools,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// AllowedTools returns the project MCP tool allowlist.
func (s *ProjectStore) AllowedTools(ctx context.Context, orgID, projectID string) ([]string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	return project.PolicyAllowedTools, nil
}

// SetRequiredApprovals stores how many distinct approvals a PR merge needs.
func (s *ProjectStore) SetRequiredApprovals(ctx context.Context, orgID, id string, n int) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectRequiredApprovals(ctx, pgstore.SetProjectRequiredApprovalsParams{
			ID: toUUID(id), OrgID: toUUID(orgID), PolicyRequiredApprovals: int32(n), //nolint:gosec // 1..3
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// SetPluginChecks stores the project plugin check JSON.
func (s *ProjectStore) SetPluginChecks(ctx context.Context, orgID, id, checks string) (domain.Project, error) {
	var out domain.Project
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).SetProjectPluginChecks(ctx, pgstore.SetProjectPluginChecksParams{
			ID: toUUID(id), OrgID: toUUID(orgID), PluginChecks: checks,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectFromRow(row)
		return nil
	})
	return out, err
}

// PluginChecks returns the project plugin check JSON.
func (s *ProjectStore) PluginChecks(ctx context.Context, orgID, projectID string) (string, error) {
	project, err := s.Get(ctx, orgID, projectID)
	if err != nil {
		return "", err
	}
	return project.PluginChecks, nil
}

// Delete removes a project and everything it owns.
func (s *ProjectStore) Delete(ctx context.Context, orgID, id string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteProject(ctx, pgstore.DeleteProjectParams{ID: toUUID(id), OrgID: toUUID(orgID)})
	})
}

// Audit records a project-domain event.
func (s *ProjectStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}
