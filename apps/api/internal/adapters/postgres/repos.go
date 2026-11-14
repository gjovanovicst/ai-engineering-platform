package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// RepoStore persists repos and pull requests.
type RepoStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewRepoStore wires the store.
func NewRepoStore(pool *pgxpool.Pool) *RepoStore {
	return &RepoStore{pool: pool, q: pgstore.New(pool)}
}

// Create inserts a repo.
func (s *RepoStore) Create(ctx context.Context, r domain.Repo) (domain.Repo, error) {
	var out domain.Repo
	err := db.WithOrg(ctx, s.pool, r.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateRepo(ctx, pgstore.CreateRepoParams{
			OrgID: toUUID(r.OrgID), ProjectID: toUUID(r.ProjectID), Provider: string(r.Provider),
			Name: r.Name, ExternalID: r.ExternalID, DefaultBranch: r.DefaultBranch,
			WebhookSecret: r.WebhookSecret,
		})
		if err != nil {
			return mapUnique(err)
		}
		out = repoFromRow(row)
		return nil
	})
	return out, err
}

// List returns the repos of a project.
func (s *RepoStore) List(ctx context.Context, orgID, projectID string) ([]domain.Repo, error) {
	var out []domain.Repo
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRepos(ctx, pgstore.ListReposParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			return err
		}
		out = make([]domain.Repo, 0, len(rows))
		for _, row := range rows {
			out = append(out, repoFromRow(row))
		}
		return nil
	})
	return out, err
}

// ByProject returns the first repo connected to a project.
func (s *RepoStore) ByProject(ctx context.Context, orgID, projectID string) (domain.Repo, error) {
	var out domain.Repo
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRepoByProject(ctx, pgstore.GetRepoByProjectParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = repoFromRow(row)
		return nil
	})
	return out, err
}

// ForWebhook resolves a repo without a session, using app_repo_org().
func (s *RepoStore) ForWebhook(ctx context.Context, repoID string) (domain.Repo, error) {
	orgID, err := s.q.RepoOrg(ctx, toUUID(repoID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Repo{}, domain.ErrNotFound
		}
		return domain.Repo{}, err
	}
	var out domain.Repo
	err = db.WithOrg(ctx, s.pool, uuidString(orgID), func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRepo(ctx, pgstore.GetRepoParams{
			ID: toUUID(repoID), OrgID: orgID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = repoFromRow(row)
		return nil
	})
	return out, err
}

// RecordPullRequest stores a PR/MR so webhooks can map it back to a task.
func (s *RepoStore) RecordPullRequest(ctx context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	var out domain.PullRequest
	err := db.WithOrg(ctx, s.pool, pr.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreatePullRequest(ctx, pgstore.CreatePullRequestParams{
			OrgID: toUUID(pr.OrgID), RepoID: toUUID(pr.RepoID), TaskID: toUUID(pr.TaskID),
			Provider: string(pr.Provider), Number: int32(pr.Number), Url: pr.URL, //nolint:gosec // PR numbers are small
		})
		if err != nil {
			return mapUnique(err)
		}
		out = pullRequestFromRow(row)
		return nil
	})
	return out, err
}

// ApprovedForPR reports whether a human approved merging this PR/MR.
func (s *RepoStore) ApprovedForPR(ctx context.Context, orgID, pullRequestID string) (bool, error) {
	found := false
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ApprovedPRApproval(ctx, pgstore.ApprovedPRApprovalParams{
			OrgID: toUUID(orgID), PullRequestID: toUUID(pullRequestID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		found = row.Valid
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// PullRequestForMerge finds the PR a webhook refers to.
func (s *RepoStore) PullRequestForMerge(ctx context.Context, orgID, repoID string, number int) (domain.PullRequest, error) {
	var out domain.PullRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).PullRequestForMerge(ctx, pgstore.PullRequestForMergeParams{
			OrgID: toUUID(orgID), RepoID: toUUID(repoID), Number: int32(number), //nolint:gosec // PR numbers are small
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = pullRequestFromRow(row)
		return nil
	})
	return out, err
}

// PullRequestsForProject lists the PRs/MRs of a project.
func (s *RepoStore) PullRequestsForProject(ctx context.Context, orgID, projectID string, limit int) ([]domain.PullRequest, error) {
	var out []domain.PullRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListPullRequestsForProject(ctx, pgstore.ListPullRequestsForProjectParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID), Limit: int32(limit), //nolint:gosec // clamped by the caller
		})
		if err != nil {
			return err
		}
		out = make([]domain.PullRequest, 0, len(rows))
		for _, row := range rows {
			out = append(out, pullRequestFromRow(row))
		}
		return nil
	})
	return out, err
}

// SetPullRequestState records the merged state.
func (s *RepoStore) SetPullRequestState(ctx context.Context, orgID, id, state string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetPullRequestState(ctx, pgstore.SetPullRequestStateParams{
			ID: toUUID(id), OrgID: toUUID(orgID), State: state,
		})
	})
}

func repoFromRow(row pgstore.Repo) domain.Repo {
	return domain.Repo{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
		Provider: domain.GitProvider(row.Provider), Name: row.Name, ExternalID: row.ExternalID,
		DefaultBranch: row.DefaultBranch, CreatedAt: row.CreatedAt.Time,
	}
}

func pullRequestFromRow(row pgstore.PullRequest) domain.PullRequest {
	return domain.PullRequest{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), RepoID: uuidString(row.RepoID),
		TaskID: uuidString(row.TaskID), Provider: domain.GitProvider(row.Provider),
		Number: int(row.Number), URL: row.Url, State: row.State,
	}
}

// PullRequest returns one PR with its task and repo labels.
func (s *RepoStore) PullRequest(ctx context.Context, orgID, id string) (domain.PullRequest, error) {
	var out domain.PullRequest
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetPullRequest(ctx, pgstore.GetPullRequestParams{
			ID: toUUID(id), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = domain.PullRequest{
			ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), RepoID: uuidString(row.RepoID),
			TaskID: uuidString(row.TaskID), Provider: domain.GitProvider(row.Provider),
			Number: int(row.Number), URL: row.Url, State: row.State,
			TaskRef: row.TaskRef, TaskTitle: row.TaskTitle, RepoName: row.RepoName,
		}
		return nil
	})
	return out, err
}

// ForWebhookName resolves a repo from a provider webhook (app-wide) by name.
func (s *RepoStore) ForWebhookName(ctx context.Context, provider, name string) (domain.Repo, error) {
	rows, err := s.pool.Query(ctx, "SELECT id::text FROM orgs ORDER BY created_at")
	if err != nil {
		return domain.Repo{}, err
	}
	var orgs []string
	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			rows.Close()
			return domain.Repo{}, err
		}
		orgs = append(orgs, orgID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.Repo{}, err
	}
	for _, orgID := range orgs {
		var out domain.Repo
		err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
			row, err := s.q.WithTx(tx).GetRepoByName(ctx, pgstore.GetRepoByNameParams{
				OrgID: toUUID(orgID), Provider: provider, Name: name,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrNotFound
				}
				return err
			}
			out = repoFromRow(row)
			return nil
		})
		if err == nil {
			return out, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.Repo{}, err
		}
	}
	return domain.Repo{}, domain.ErrNotFound
}

// ForWebhookNameInOrg resolves a repo inside one org (per-org app webhooks).
func (s *RepoStore) ForWebhookNameInOrg(ctx context.Context, orgID, provider, name string) (domain.Repo, error) {
	var out domain.Repo
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRepoByName(ctx, pgstore.GetRepoByNameParams{
			OrgID: toUUID(orgID), Provider: provider, Name: name,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = repoFromRow(row)
		return nil
	})
	return out, err
}
