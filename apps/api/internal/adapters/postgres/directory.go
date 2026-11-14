package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/directory"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// DirectoryStore implements directory.Store.
type DirectoryStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewDirectoryStore wires the store.
func NewDirectoryStore(pool *pgxpool.Pool) *DirectoryStore {
	return &DirectoryStore{pool: pool, q: pgstore.New(pool)}
}

// Repos lists connected repositories with project names.
func (s *DirectoryStore) Repos(ctx context.Context, orgID string, limit int) ([]directory.Repo, error) {
	var out []directory.Repo
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListOrgRepos(ctx, pgstore.ListOrgReposParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]directory.Repo, 0, len(rows))
		for _, row := range rows {
			out = append(out, directory.Repo{
				ID: uuidString(row.ID), ProjectID: uuidString(row.ProjectID), ProjectName: row.ProjectName,
				Provider: row.Provider, Name: row.Name, DefaultBranch: row.DefaultBranch, CreatedAt: row.CreatedAt.Time,
			})
		}
		return nil
	})
	return out, err
}

// Runs lists runs with optional filters.
func (s *DirectoryStore) Runs(ctx context.Context, orgID, status, agentType string, limit int) ([]directory.Run, error) {
	var out []directory.Run
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		params := pgstore.ListOrgRunsParams{
			OrgID: toUUID(orgID), Column2: status, Column3: agentType,
			Limit: int32(limit), //nolint:gosec // clamped by the service
		}
		rows, err := s.q.WithTx(tx).ListOrgRuns(ctx, params)
		if err != nil {
			return err
		}
		out = make([]directory.Run, 0, len(rows))
		for _, row := range rows {
			run := directory.Run{
				ID: uuidString(row.ID), ProjectID: uuidString(row.ProjectID),
				TaskRef: row.TaskRef, TaskTitle: row.TaskTitle,
				AgentType: row.AgentType, Status: row.Status, CreatedAt: row.CreatedAt.Time,
			}
			if row.FinishedAt.Valid {
				v := row.FinishedAt.Time
				run.FinishedAt = &v
			}
			out = append(out, run)
		}
		return nil
	})
	return out, err
}

// Members lists workspace members.
func (s *DirectoryStore) Members(ctx context.Context, orgID string, limit int) ([]directory.Member, error) {
	var out []directory.Member
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListMembers(ctx, pgstore.ListMembersParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]directory.Member, 0, len(rows))
		for _, row := range rows {
			out = append(out, directory.Member{
				UserID: uuidString(row.UserID), Email: row.Email, Name: row.Name,
				Role: row.Role, CreatedAt: row.CreatedAt.Time,
			})
		}
		return nil
	})
	return out, err
}

// Audit lists the audit log.
func (s *DirectoryStore) Audit(ctx context.Context, orgID string, limit int) ([]directory.AuditEntry, error) {
	var out []directory.AuditEntry
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListAudit(ctx, pgstore.ListAuditParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]directory.AuditEntry, 0, len(rows))
		for _, row := range rows {
			entry := directory.AuditEntry{
				ID: row.ID, ActorKind: row.ActorKind, Action: row.Action,
				Target: row.Target, Metadata: json.RawMessage(row.Metadata),
				PrevHash: row.PrevHash, Hash: row.Hash, CreatedAt: row.CreatedAt.Time,
			}
			if row.ActorID.Valid {
				entry.ActorID = uuidString(row.ActorID)
			}
			out = append(out, entry)
		}
		return nil
	})
	return out, err
}

// UpdateRole changes a membership role.
func (s *DirectoryStore) UpdateRole(ctx context.Context, orgID, userID, role string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.q.WithTx(tx).UpdateMemberRole(ctx, pgstore.UpdateMemberRoleParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Role: role,
		})
		return err
	})
}

// RemoveMember deletes a membership.
func (s *DirectoryStore) RemoveMember(ctx context.Context, orgID, userID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteMember(ctx, pgstore.DeleteMemberParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID),
		})
	})
}

// RecordAudit appends an audit entry.
func (s *DirectoryStore) RecordAudit(ctx context.Context, orgID, actorID, action, target string, metadata []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).RecordAudit(ctx, pgstore.RecordAuditParams{
			OrgID: toUUID(orgID), ActorID: toUUID(actorID), Action: action, Target: target, Metadata: metadata,
		})
	})
}
