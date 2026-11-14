package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/analytics"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// AnalyticsStore reads org metrics.
type AnalyticsStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewAnalyticsStore wires the store.
func NewAnalyticsStore(pool *pgxpool.Pool) *AnalyticsStore {
	return &AnalyticsStore{pool: pool, q: pgstore.New(pool)}
}

// RunsByStatus counts runs per status.
func (s *AnalyticsStore) RunsByStatus(ctx context.Context, orgID string, days int) (map[string]int, error) {
	out := map[string]int{}
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).CountRunsByStatus(ctx, pgstore.CountRunsByStatusParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out[row.Status] = int(row.Total)
		}
		return nil
	})
	return out, err
}

// RunDurationSeconds returns the average successful run duration.
func (s *AnalyticsStore) RunDurationSeconds(ctx context.Context, orgID string, days int) (float64, int, error) {
	var avg float64
	var count int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).RunDurationStats(ctx, pgstore.RunDurationStatsParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		avg, count = row.AvgSeconds, int(row.Total)
		return nil
	})
	return avg, count, err
}

// ApprovalDecisionSeconds returns the average time to decide approvals.
func (s *AnalyticsStore) ApprovalDecisionSeconds(ctx context.Context, orgID string, days int) (float64, int, error) {
	var avg float64
	var count int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ApprovalDecisionStats(ctx, pgstore.ApprovalDecisionStatsParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		avg, count = row.AvgSeconds, int(row.Total)
		return nil
	})
	return avg, count, err
}

// AvgRunsPerTask returns rework: runs per task.
func (s *AnalyticsStore) AvgRunsPerTask(ctx context.Context, orgID string) (float64, error) {
	var avg float64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).RunsPerTaskStats(ctx, toUUID(orgID))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		avg = row
		return nil
	})
	return avg, err
}

// UsageCost sums the usage ledger for the window.
func (s *AnalyticsStore) UsageCost(ctx context.Context, orgID string, days int) (float64, int, error) {
	var cost float64
	var rows int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).UsageCost(ctx, pgstore.UsageCostParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		cost, rows = row.Cost, int(row.Rows)
		return nil
	})
	return cost, rows, err
}

// MergedPullRequests counts merged PRs/MRs.
func (s *AnalyticsStore) MergedPullRequests(ctx context.Context, orgID string, days int) (int, error) {
	var total int
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).PullRequestsMerged(ctx, pgstore.PullRequestsMergedParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		total = int(row)
		return nil
	})
	return total, err
}

// OnboardingSeconds averages project-creation to first merged PR.
func (s *AnalyticsStore) OnboardingSeconds(ctx context.Context, orgID string, days int) (float64, int, error) {
	var avg float64
	var projects int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).OnboardingSeconds(ctx, pgstore.OnboardingSecondsParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		avg, projects = row.AvgSeconds, row.Projects
		return nil
	})
	return avg, int(projects), err
}

// MergedPerDay returns merged PR counts per day.
func (s *AnalyticsStore) MergedPerDay(ctx context.Context, orgID string, days int) ([]analytics.DayPoint, error) {
	var out []analytics.DayPoint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).MergedPerDay(ctx, pgstore.MergedPerDayParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, analytics.DayPoint{Date: row.Day, Total: int(row.Total)})
		}
		return nil
	})
	return out, err
}

// LeadTimePerDay returns average merge lead time per day.
func (s *AnalyticsStore) LeadTimePerDay(ctx context.Context, orgID string, days int) ([]analytics.LeadTimePoint, error) {
	var out []analytics.LeadTimePoint
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).LeadTimePerDay(ctx, pgstore.LeadTimePerDayParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, analytics.LeadTimePoint{Date: row.Day, Minutes: row.Minutes})
		}
		return nil
	})
	return out, err
}

// Incidents counts incidents opened in the window.
func (s *AnalyticsStore) Incidents(ctx context.Context, orgID string, days int) (int, error) {
	var count int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).IncidentCountWindow(ctx, pgstore.IncidentCountWindowParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		count = n
		return err
	})
	return int(count), err
}

// MTTRSeconds returns average resolution time and resolved count.
func (s *AnalyticsStore) MTTRSeconds(ctx context.Context, orgID string, days int) (float64, int, error) {
	var seconds float64
	var count int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).IncidentMTTRSeconds(ctx, pgstore.IncidentMTTRSecondsParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		seconds, count = row.Column1, row.Column2
		return err
	})
	return seconds, int(count), err
}

// FirstPassMergeRate is the share of merged PRs whose task had one succeeded dev run.
func (s *AnalyticsStore) FirstPassMergeRate(ctx context.Context, orgID string, days int) (float64, error) {
	var out float64
	window := int32(days) //nolint:gosec // clamped by the service
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		value, err := s.q.WithTx(tx).FirstPassMergeRate(ctx, pgstore.FirstPassMergeRateParams{OrgID: toUUID(orgID), Column2: window})
		if err != nil {
			return err
		}
		out = value
		return nil
	})
	return out, err
}

// ProofValidity is the share of succeeded development runs with acceptance proof.
func (s *AnalyticsStore) ProofValidity(ctx context.Context, orgID string, days int) (float64, int64, error) {
	var share float64
	var total int64
	window := int32(days) //nolint:gosec // clamped by the service
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).ProofValidity(ctx, pgstore.ProofValidityParams{OrgID: toUUID(orgID), Column2: window})
		if err != nil {
			return err
		}
		share, total = row.Share, row.Total
		return nil
	})
	return share, total, err
}

// DevRunsPerTask is the average number of development runs per task.
func (s *AnalyticsStore) DevRunsPerTask(ctx context.Context, orgID string, days int) (float64, error) {
	var out float64
	window := int32(days) //nolint:gosec // clamped by the service
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		value, err := s.q.WithTx(tx).DevRunsPerTask(ctx, pgstore.DevRunsPerTaskParams{OrgID: toUUID(orgID), Column2: window})
		if err != nil {
			return err
		}
		out = value
		return nil
	})
	return out, err
}

// ProjectDORA returns per-project merge/incident/MTTR rows.
func (s *AnalyticsStore) ProjectDORA(ctx context.Context, orgID string, days int) ([]analytics.ProjectDORA, error) {
	var out []analytics.ProjectDORA
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ProjectDORA(ctx, pgstore.ProjectDORAParams{
			OrgID: toUUID(orgID), Column2: int32(days), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			entry := analytics.ProjectDORA{
				ProjectID: uuidString(row.ID), Name: row.Name, Merged: int(row.Merges),
				Incidents: int(row.Incidents), MTTR: row.MttrSeconds,
			}
			if row.FirstMergedAt.Valid {
				hours := row.FirstMergedAt.Time.Sub(row.CreatedAt.Time).Hours()
				entry.OnboardHours = &hours
			}
			for _, step := range []struct {
				at  pgtype.Timestamptz
				dst **float64
			}{
				{row.FirstTaskAt, &entry.TaskHours},
				{row.FirstRunAt, &entry.RunHours},
				{row.FirstPrAt, &entry.PRHours},
			} {
				if step.at.Valid {
					hours := step.at.Time.Sub(row.CreatedAt.Time).Hours()
					*step.dst = &hours
				}
			}
			out = append(out, entry)
		}
		return nil
	})
	return out, err
}
