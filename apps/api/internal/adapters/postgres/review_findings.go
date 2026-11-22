package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// ReviewFindingStore persists the review findings and their lifecycle.
type ReviewFindingStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewReviewFindingStore builds the store.
func NewReviewFindingStore(pool *pgxpool.Pool) *ReviewFindingStore {
	return &ReviewFindingStore{pool: pool, q: pgstore.New(pool)}
}

// reviewFindingFromRow maps a row to the domain type.
func reviewFindingFromRow(row pgstore.ReviewFinding) domain.ReviewFinding {
	return domain.ReviewFinding{
		ID:         uuidString(row.ID),
		OrgID:      uuidString(row.OrgID),
		ProjectID:  uuidString(row.ProjectID),
		TaskID:     uuidString(row.TaskID),
		RunID:      uuidString(row.RunID),
		Severity:   domain.ReviewFindingSeverity(row.Severity),
		File:       row.File,
		Line:       int(row.Line),
		Message:    row.Message,
		Status:     domain.ReviewFindingStatus(row.Status),
		DecidedBy:  uuidString(row.DecidedBy),
		Reason:     row.Reason,
		FixedByRun: uuidString(row.FixedByRun),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

// safeLine clamps a line number into the column's range.
func safeLine(line int) int32 {
	return int32(min(max(line, 0), math.MaxInt32))
}

// List returns the task's findings, oldest first.
func (s *ReviewFindingStore) List(ctx context.Context, orgID, taskID string) ([]domain.ReviewFinding, error) {
	var out []domain.ReviewFinding
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListReviewFindings(ctx, pgstore.ListReviewFindingsParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, reviewFindingFromRow(row))
		}
		return nil
	})
	return out, err
}

// Reconcile records one review's findings against the task's existing ones. A
// finding that matches an existing row (file + message) keeps its decision: a
// dismissed nit stays dismissed, a fixed one that re-appears re-opens. A
// previously open row the new review no longer reports becomes fixed.
func (s *ReviewFindingStore) Reconcile(ctx context.Context, orgID, projectID, taskID, runID string, findings []domain.ReviewFinding) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		existing, err := q.ListReviewFindings(ctx, pgstore.ListReviewFindingsParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		if err != nil {
			return err
		}
		byKey := map[string]pgstore.ReviewFinding{}
		for _, row := range existing {
			byKey[row.File+"\x00"+row.Message] = row
		}
		reported := map[string]bool{}
		for _, finding := range findings {
			key := finding.File + "\x00" + finding.Message
			reported[key] = true
			if row, ok := byKey[key]; ok {
				if err := q.TouchReviewFinding(ctx, pgstore.TouchReviewFindingParams{
					OrgID: toUUID(orgID), ID: row.ID, RunID: toUUID(runID), Severity: string(finding.Severity),
				}); err != nil {
					return err
				}
				if domain.ReviewFindingStatus(row.Status) == domain.ReviewFindingFixed {
					// The fix did not hold: reopen it.
					if _, err := q.DecideReviewFinding(ctx, pgstore.DecideReviewFindingParams{
						OrgID: toUUID(orgID), ID: row.ID, Status: string(domain.ReviewFindingOpen),
					}); err != nil {
						return err
					}
				}
				continue
			}
			if _, err := q.InsertReviewFinding(ctx, pgstore.InsertReviewFindingParams{
				OrgID: toUUID(orgID), ProjectID: toUUID(projectID), TaskID: toUUID(taskID), RunID: toUUID(runID),
				Severity: string(finding.Severity), File: finding.File, Line: safeLine(finding.Line), Message: finding.Message,
			}); err != nil {
				return err
			}
		}
		// An open finding the new review no longer reports is fixed.
		for key, row := range byKey {
			if reported[key] || domain.ReviewFindingStatus(row.Status) != domain.ReviewFindingOpen {
				continue
			}
			if err := q.MarkReviewFindingFixed(ctx, pgstore.MarkReviewFindingFixedParams{
				OrgID: toUUID(orgID), ID: row.ID, FixedByRun: toUUID(runID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Decide dismisses or reopens a finding. A dismissal needs a reason.
func (s *ReviewFindingStore) Decide(ctx context.Context, orgID, id, status, by, reason string) (domain.ReviewFinding, error) {
	var out domain.ReviewFinding
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).DecideReviewFinding(ctx, pgstore.DecideReviewFindingParams{
			OrgID: toUUID(orgID), ID: toUUID(id), Status: status, DecidedBy: toUUID(by), Reason: reason,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = reviewFindingFromRow(row)
		return nil
	})
	return out, err
}

// OpenBlockers counts the open blockers of a task.
func (s *ReviewFindingStore) OpenBlockers(ctx context.Context, orgID, taskID string) (int, error) {
	var count int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountOpenReviewBlockers(ctx, pgstore.CountOpenReviewBlockersParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		count = n
		return err
	})
	return int(count), err
}
