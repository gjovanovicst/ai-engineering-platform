package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/agentplugins"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AgentPluginStore implements agentplugins.Store.
type AgentPluginStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAgentPluginStore wires the store.
func NewAgentPluginStore(pool *pgxpool.Pool) *AgentPluginStore {
	return &AgentPluginStore{pool: pool, q: pgstore.New(pool)}
}

func (s *AgentPluginStore) Create(ctx context.Context, orgID, actorID, name, version, image, command string, types []string) (agentplugins.Plugin, error) {
	var out agentplugins.Plugin
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateAgentPlugin(ctx, pgstore.CreateAgentPluginParams{
			OrgID: toUUID(orgID), Name: name, Version: version, Image: image, Command: command,
			AgentTypes: types, CreatedBy: toOptionalUUID(&actorID),
		})
		if err != nil {
			return err
		}
		out = agentplugins.Plugin{ID: uuidString(row.ID), Name: row.Name, Version: row.Version, Image: row.Image, Command: row.Command, AgentTypes: row.AgentTypes, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}

func (s *AgentPluginStore) List(ctx context.Context, orgID string) ([]agentplugins.Plugin, error) {
	var out []agentplugins.Plugin
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListAgentPlugins(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, agentplugins.Plugin{ID: uuidString(row.ID), Name: row.Name, Version: row.Version, Image: row.Image, Command: row.Command, AgentTypes: row.AgentTypes, CreatedAt: row.CreatedAt.Time})
		}
		return nil
	})
	return out, err
}

func (s *AgentPluginStore) SetProjectPlugin(ctx context.Context, orgID, projectID, pluginID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var id *string
		if pluginID != "" {
			id = &pluginID
		}
		_, err := s.q.WithTx(tx).SetProjectAgentPlugin(ctx, pgstore.SetProjectAgentPluginParams{
			ID: toUUID(projectID), OrgID: toUUID(orgID), AgentPluginID: toOptionalUUID(id),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		return nil
	})
}

func (s *AgentPluginStore) ForProject(ctx context.Context, orgID, projectID string) (agentplugins.Plugin, error) {
	var out agentplugins.Plugin
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ProjectAgentPlugin(ctx, pgstore.ProjectAgentPluginParams{ID: toUUID(projectID), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = agentplugins.Plugin{ID: uuidString(row.ID), Name: row.Name, Version: row.Version, Image: row.Image, Command: row.Command, AgentTypes: row.AgentTypes}
		return nil
	})
	return out, err
}

func (s *AgentPluginStore) Audit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordOrgAudit(ctx, pgstore.RecordOrgAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}

var _ = time.Time{}
