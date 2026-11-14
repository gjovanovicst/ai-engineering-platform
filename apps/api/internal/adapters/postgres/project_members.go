package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projectaccess"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// ProjectMemberStore implements projectaccess.Store.
type ProjectMemberStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewProjectMemberStore wires the store.
func NewProjectMemberStore(pool *pgxpool.Pool) *ProjectMemberStore {
	return &ProjectMemberStore{pool: pool, q: pgstore.New(pool)}
}

func (s *ProjectMemberStore) Upsert(ctx context.Context, orgID, projectID, userID, role string) (projectaccess.Member, error) {
	var out projectaccess.Member
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertProjectMember(ctx, pgstore.UpsertProjectMemberParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID), UserID: toUUID(userID), Role: role,
		})
		if err != nil {
			return err
		}
		out = projectaccess.Member{UserID: uuidString(row.UserID), Role: row.Role, CreatedAt: row.CreatedAt.Time}
		return nil
	})
	return out, err
}

func (s *ProjectMemberStore) List(ctx context.Context, orgID, projectID string) ([]projectaccess.Member, error) {
	var out []projectaccess.Member
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListProjectMembers(ctx, pgstore.ListProjectMembersParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, projectaccess.Member{UserID: uuidString(row.UserID), Role: row.Role, CreatedAt: row.CreatedAt.Time})
		}
		return nil
	})
	return out, err
}

func (s *ProjectMemberStore) Delete(ctx context.Context, orgID, projectID, userID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).DeleteProjectMember(ctx, pgstore.DeleteProjectMemberParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID), UserID: toUUID(userID),
		})
	})
}

func (s *ProjectMemberStore) Access(ctx context.Context, orgID, projectID, userID string) (projectaccess.Access, error) {
	var out projectaccess.Access
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ProjectAccess(ctx, pgstore.ProjectAccessParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID), UserID: toUUID(userID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = projectaccess.Access{Role: row.Role, Members: int(row.Members)}
		return nil
	})
	return out, err
}
