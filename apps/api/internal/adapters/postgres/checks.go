package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// CheckStore implements checks.Store.
type CheckStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewCheckStore wires the store.
func NewCheckStore(pool *pgxpool.Pool) *CheckStore {
	return &CheckStore{pool: pool, q: pgstore.New(pool)}
}

// Upsert inserts or updates a check result.
func (s *CheckStore) Upsert(ctx context.Context, orgID, repoID, prID, runID string, c checks.Check) (checks.Check, error) {
	var out checks.Check
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UpsertCheckResult(ctx, pgstore.UpsertCheckResultParams{
			OrgID: toUUID(orgID), RepoID: optionalUUID(repoID), PullRequestID: optionalUUID(prID),
			RunID: optionalUUID(runID), Name: c.Name, Status: pgstore.CheckStatus(c.Status),
			Output: c.Output, DurationMs: int32(c.DurationMS), CommitSha: c.CommitSHA, Url: c.URL, //nolint:gosec // durations
		})
		if err != nil {
			return err
		}
		out = checkFromRow(row)
		return nil
	})
	return out, err
}

// ListForPullRequest lists checks of a PR.
func (s *CheckStore) ListForPullRequest(ctx context.Context, orgID, prID string) ([]checks.Check, error) {
	var out []checks.Check
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListChecksForPullRequest(ctx, pgstore.ListChecksForPullRequestParams{
			OrgID: toUUID(orgID), PullRequestID: toUUID(prID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, checkFromRow(row))
		}
		return nil
	})
	return out, err
}

// ListForRun lists checks of a run.
func (s *CheckStore) ListForRun(ctx context.Context, orgID, runID string) ([]checks.Check, error) {
	var out []checks.Check
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListChecksForRun(ctx, pgstore.ListChecksForRunParams{
			OrgID: toUUID(orgID), RunID: toUUID(runID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, checkFromRow(row))
		}
		return nil
	})
	return out, err
}

// InsertForRun stores a run's own test result.
func (s *CheckStore) InsertForRun(ctx context.Context, orgID, runID string, c checks.Check) (checks.Check, error) {
	var out checks.Check
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).InsertRunCheck(ctx, pgstore.InsertRunCheckParams{
			OrgID: toUUID(orgID), RunID: optionalUUID(runID), Name: c.Name,
			Status: pgstore.CheckStatus(c.Status), Output: c.Output, Url: c.URL,
		})
		if err != nil {
			return err
		}
		out = checkFromRow(row)
		return nil
	})
	return out, err
}

// RequiredForPullRequest returns the project's required check names.
func (s *CheckStore) RequiredForPullRequest(ctx context.Context, orgID, prID string) ([]string, error) {
	var out []string
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).RequiredChecksForPR(ctx, pgstore.RequiredChecksForPRParams{
			ID: toUUID(prID), OrgID: toUUID(orgID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

// Failed reports whether any check failed.
func (s *CheckStore) Failed(ctx context.Context, orgID, prID string) (bool, error) {
	failed := false
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountFailedChecks(ctx, pgstore.CountFailedChecksParams{
			OrgID: toUUID(orgID), PullRequestID: toUUID(prID),
		})
		if err != nil {
			return err
		}
		failed = n > 0
		return nil
	})
	return failed, err
}

func checkFromRow(row pgstore.TestResult) checks.Check {
	return checks.Check{
		ID: uuidString(row.ID), Name: row.Name, Status: string(row.Status),
		Output: row.Output, DurationMS: int(row.DurationMs), CommitSHA: row.CommitSha,
		URL: row.Url,
	}
}

var _ checks.Store = (*CheckStore)(nil)
