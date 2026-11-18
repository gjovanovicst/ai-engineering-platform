package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// RunStore implements runs.Store on PostgreSQL.
type RunStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
	pub  Publisher
}

// Publisher notifies subscribers that a run changed (Redis pub/sub).
type Publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

// WithPublisher attaches the fan-out publisher.
func (s *RunStore) WithPublisher(pub Publisher) *RunStore {
	s.pub = pub
	return s
}

// NewRunStore wires the store.
func NewRunStore(pool *pgxpool.Pool) *RunStore {
	return &RunStore{pool: pool, q: pgstore.New(pool)}
}

var _ runs.Store = (*RunStore)(nil)

// Create inserts the run and its first event in one transaction.
func (s *RunStore) Create(ctx context.Context, r domain.Run) (domain.Run, error) {
	var out domain.Run
	err := db.WithOrg(ctx, s.pool, r.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		budget, _ := json.Marshal(r.Budget)
		row, err := q.CreateRun(ctx, pgstore.CreateRunParams{
			OrgID: toUUID(r.OrgID), ProjectID: toUUID(r.ProjectID), TaskID: toUUID(r.TaskID),
			AgentType: r.AgentType, Model: r.Model, Budget: budget, FrozenContextHash: r.FrozenContextHash, FrozenContextSources: r.FrozenContextSources,
			FixOfRunID: toUUID(r.FixOfRunID),
			FixKind:    r.FixKind, FixContext: optionalString(r.FixContext),
		})
		if err != nil {
			return mapUnique(err)
		}
		out = runFromRow(row)
		payload, _ := json.Marshal(map[string]any{"agent_type": r.AgentType, "model": r.Model})
		_, err = q.InsertRunEvent(ctx, pgstore.InsertRunEventParams{
			RunID: row.ID, Type: "run.queued", Actor: "system", Payload: payload,
		})
		return err
	})
	return out, err
}

// Get returns one run.
// SetContextHash records the manifest hash built for the run.
func (s *RunStore) SetContextHash(ctx context.Context, orgID, runID, hash string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetRunContextHash(ctx, pgstore.SetRunContextHashParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), ContextHash: hash,
		})
	})
}

func (s *RunStore) Get(ctx context.Context, orgID, id string) (domain.Run, error) {
	var out domain.Run
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetRun(ctx, pgstore.GetRunParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = runFromRow(row)
		return nil
	})
	return out, err
}

// ListForTask returns the runs of a task, newest first.
func (s *RunStore) ListForTask(ctx context.Context, orgID, taskID string, limit int) ([]domain.Run, error) {
	var out []domain.Run
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRunsForTask(ctx, pgstore.ListRunsForTaskParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID), Limit: int32(limit), //nolint:gosec // clamped by the service
		})
		if err != nil {
			return err
		}
		out = make([]domain.Run, 0, len(rows))
		for _, row := range rows {
			out = append(out, runFromRow(row))
		}
		return nil
	})
	return out, err
}

// UpdateStatus persists the state-machine result and appends an event.
func (s *RunStore) UpdateStatus(ctx context.Context, r domain.Run) (domain.Run, error) {
	var out domain.Run
	err := db.WithOrg(ctx, s.pool, r.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.UpdateRunStatus(ctx, pgstore.UpdateRunStatusParams{
			ID: toUUID(r.ID), OrgID: toUUID(r.OrgID), Status: string(r.Status),
			CurrentPhase: nilIfEmpty(string(r.CurrentPhase)),
			FailureCode:  nilIfEmpty(r.FailureCode), FailureDetail: nilIfEmpty(r.FailureDetail),
			CancelRequestedAt: timePtr(r.CancelRequestedAt),
			StartedAt:         timePtr(r.StartedAt), FinishedAt: timePtr(r.FinishedAt),
		})
		if err != nil {
			return err
		}
		out = runFromRow(row)
		payload, _ := json.Marshal(map[string]any{"status": r.Status})
		_, err = q.InsertRunEvent(ctx, pgstore.InsertRunEventParams{
			RunID: row.ID, Type: "run." + string(r.Status), Actor: "system", Payload: payload,
		})
		return err
	})
	return out, err
}

// AppendEvent adds one event with a per-run sequence.
func (s *RunStore) AppendEvent(ctx context.Context, orgID, runID, eventType, actor string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var seq int64
	err = db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.LockRun(ctx, pgstore.LockRunParams{ID: toUUID(runID), OrgID: toUUID(orgID)}); err != nil {
			return err
		}
		row, err := q.InsertRunEvent(ctx, pgstore.InsertRunEventParams{
			RunID: toUUID(runID), Type: eventType, Actor: actor, Payload: raw,
		})
		if err != nil {
			return err
		}
		seq = row.Seq
		return nil
	})
	if err != nil {
		return err
	}
	if s.pub != nil {
		// Best effort: the database is the source of truth and the SSE handler
		// falls back to polling.
		_ = s.pub.Publish(context.WithoutCancel(ctx), "run:"+runID, []byte(strconv.FormatInt(seq, 10)))
	}
	return nil
}

// EventsAfter returns events with seq greater than after (SSE replay).
func (s *RunStore) EventsAfter(ctx context.Context, orgID, runID string, after int64) ([]runs.Event, error) {
	var out []runs.Event
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRunEventsAfter(ctx, pgstore.ListRunEventsAfterParams{
			RunID: toUUID(runID), Seq: after,
		})
		if err != nil {
			return err
		}
		out = make([]runs.Event, 0, len(rows))
		for _, row := range rows {
			var payload map[string]any
			_ = json.Unmarshal(row.Payload, &payload)
			out = append(out, runs.Event{
				Seq: row.Seq, Type: row.Type, Actor: row.Actor,
				Payload: payload, CreatedAt: row.CreatedAt.Time,
			})
		}
		return nil
	})
	return out, err
}

// Events returns the full event log of a run.
func (s *RunStore) Events(ctx context.Context, orgID, runID string) ([]runs.Event, error) {
	var out []runs.Event
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListRunEvents(ctx, toUUID(runID))
		if err != nil {
			return err
		}
		out = make([]runs.Event, 0, len(rows))
		for _, row := range rows {
			var payload map[string]any
			_ = json.Unmarshal(row.Payload, &payload)
			out = append(out, runs.Event{
				Seq: row.Seq, Type: row.Type, Actor: row.Actor,
				Payload: payload, CreatedAt: row.CreatedAt.Time,
			})
		}
		return nil
	})
	return out, err
}

func runFromRow(row pgstore.Run) domain.Run {
	r := domain.Run{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), ProjectID: uuidString(row.ProjectID),
		TaskID: uuidString(row.TaskID), AgentType: row.AgentType, Status: domain.RunStatus(row.Status),
		Model: row.Model, Plan: row.Plan, CreatedAt: row.CreatedAt.Time,
		ContextHash: row.ContextHash, FrozenContextHash: row.FrozenContextHash,
		FrozenContextSources: row.FrozenContextSources, TraceID: row.TraceID,
	}
	if row.FixOfRunID.Valid {
		r.FixOfRunID = uuidString(row.FixOfRunID)
	}
	r.FixKind = row.FixKind
	if row.FixContext != nil {
		r.FixContext = []byte(*row.FixContext)
	}
	if len(row.Budget) > 0 {
		_ = json.Unmarshal(row.Budget, &r.Budget)
	}
	r.Review = row.Review
	r.AcceptanceProof = row.AcceptanceProof
	if row.Diff != nil {
		r.Diff = []byte(*row.Diff)
	}
	if row.CurrentPhase != nil {
		r.CurrentPhase = domain.PhaseName(*row.CurrentPhase)
	}
	if row.FailureCode != nil {
		r.FailureCode = *row.FailureCode
	}
	if row.FailureDetail != nil {
		r.FailureDetail = *row.FailureDetail
	}
	if row.StartedAt.Valid {
		v := row.StartedAt.Time
		r.StartedAt = &v
	}
	if row.FinishedAt.Valid {
		v := row.FinishedAt.Time
		r.FinishedAt = &v
	}
	if row.CancelRequestedAt.Valid {
		v := row.CancelRequestedAt.Time
		r.CancelRequestedAt = &v
	}
	return r
}

// SetPlan stores the plan and its hash.
func (s *RunStore) SetPlan(ctx context.Context, orgID, runID string, plan []byte, sha string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetRunPlan(ctx, pgstore.SetRunPlanParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), Plan: plan, PlanSha256: &sha,
		})
	})
}

// SetDiff stores the produced diff and its hash.
func (s *RunStore) SetDiff(ctx context.Context, orgID, runID string, diff []byte, sha string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		text := string(diff)
		return s.q.WithTx(tx).SetRunDiff(ctx, pgstore.SetRunDiffParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), Diff: &text, DiffSha256: &sha,
		})
	})
}

// SetReview stores the review findings.
func (s *RunStore) SetReview(ctx context.Context, orgID, runID string, review []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetRunReview(ctx, pgstore.SetRunReviewParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), Review: review,
		})
	})
}

// LatestRun returns the newest succeeded run of a given type for a task.
func (s *RunStore) LatestRun(ctx context.Context, orgID, taskID, agentType string) (domain.Run, error) {
	var out domain.Run
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).LatestRunForTask(ctx, pgstore.LatestRunForTaskParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID), AgentType: agentType,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = runFromRow(row)
		return nil
	})
	return out, err
}

// CreateApproval opens a pending approval for the run.
func (s *RunStore) CreateApproval(ctx context.Context, orgID, runID, pullRequestID, kind string, expires time.Time) (runs.Approval, error) {
	var out runs.Approval
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).CreateApproval(ctx, pgstore.CreateApprovalParams{
			OrgID: toUUID(orgID), RunID: toUUID(runID), PullRequestID: toUUID(pullRequestID),
			Type: kind, ExpiresAt: toTS(expires),
		})
		if err != nil {
			return err
		}
		out = approvalFromRow(row)
		return nil
	})
	return out, err
}

// GetApproval returns one approval.
func (s *RunStore) GetApproval(ctx context.Context, orgID, id string) (runs.Approval, error) {
	var out runs.Approval
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).GetApproval(ctx, pgstore.GetApprovalParams{ID: toUUID(id), OrgID: toUUID(orgID)})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = approvalFromRow(pgstore.Approval{
			ID: row.ID, OrgID: row.OrgID, RunID: row.RunID, Type: row.Type, Status: row.Status,
			RequestedAt: row.RequestedAt, DecidedAt: row.DecidedAt, DecidedBy: row.DecidedBy,
			Reason: row.Reason, ExpiresAt: row.ExpiresAt, PullRequestID: row.PullRequestID,
		})
		if row.PrNumber != nil {
			out.Number = int(*row.PrNumber)
		}
		if row.PrUrl != nil {
			out.URL = *row.PrUrl
		}
		return nil
	})
	return out, err
}

// DecideApproval records the decision if the approval is still pending.
func (s *RunStore) DecideApproval(ctx context.Context, orgID, id, status, reason, userID string, at time.Time) (runs.Approval, error) {
	var out runs.Approval
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).DecideApproval(ctx, pgstore.DecideApprovalParams{
			ID: toUUID(id), OrgID: toUUID(orgID), Status: status,
			DecidedAt: toTS(at), DecidedBy: toUUID(userID), Reason: nilIfEmpty(reason),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTransition
			}
			return err
		}
		out = approvalFromRow(row)
		return nil
	})
	return out, err
}

// PendingApprovals lists pending approvals of an org.
// CountActive counts queued and running runs of the org.
func (s *RunStore) CountActive(ctx context.Context, orgID string) (int, error) {
	count := 0
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountActiveRuns(ctx, toUUID(orgID))
		count = int(n)
		return err
	})
	return count, err
}

// ExpireApprovals marks pending approvals past their deadline as expired.
func (s *RunStore) ExpireApprovals(ctx context.Context, orgID string) ([]runs.ExpiredApproval, error) {
	var out []runs.ExpiredApproval
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ExpireApprovals(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, runs.ExpiredApproval{
				ID: uuidString(row.ID), RunID: uuidString(row.RunID), Type: row.Type,
			})
		}
		return nil
	})
	return out, err
}

func (s *RunStore) PendingApprovals(ctx context.Context, orgID string, limit int) ([]runs.Approval, error) {
	var out []runs.Approval
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListPendingApprovals(ctx, pgstore.ListPendingApprovalsParams{
			OrgID: toUUID(orgID), Limit: int32(limit), //nolint:gosec // clamped by the caller
		})
		if err != nil {
			return err
		}
		out = make([]runs.Approval, 0, len(rows))
		for _, row := range rows {
			a := approvalFromRow(pgstore.Approval{
				ID: row.ID, OrgID: row.OrgID, RunID: row.RunID, Type: row.Type, Status: row.Status,
				RequestedAt: row.RequestedAt, DecidedAt: row.DecidedAt, DecidedBy: row.DecidedBy,
				Reason: row.Reason, ExpiresAt: row.ExpiresAt, PullRequestID: row.PullRequestID,
			})
			if row.PrNumber != nil {
				a.Number = int(*row.PrNumber)
			}
			if row.PrUrl != nil {
				a.URL = *row.PrUrl
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

func approvalFromRow(row pgstore.Approval) runs.Approval {
	a := runs.Approval{
		ID: uuidString(row.ID), OrgID: uuidString(row.OrgID), RunID: uuidString(row.RunID),
		Type: row.Type, Status: row.Status, RequestedAt: row.RequestedAt.Time,
	}
	if row.DecidedAt.Valid {
		v := row.DecidedAt.Time
		a.DecidedAt = &v
	}
	if row.DecidedBy.Valid {
		a.DecidedBy = uuidString(row.DecidedBy)
	}
	if row.Reason != nil {
		a.Reason = *row.Reason
	}
	if row.PullRequestID.Valid {
		a.PullRequestID = uuidString(row.PullRequestID)
	}
	return a
}

// LatestReviewForTask returns the newest review JSON of a task.
func (s *RunStore) LatestReviewForTask(ctx context.Context, orgID, taskID string) ([]byte, error) {
	var out []byte
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		review, err := s.q.WithTx(tx).LatestReviewForTask(ctx, pgstore.LatestReviewForTaskParams{
			OrgID: toUUID(orgID), TaskID: toUUID(taskID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		out = review
		return nil
	})
	return out, err
}

// SetAcceptanceProof stores the validated proof on the run.
func (s *RunStore) SetAcceptanceProof(ctx context.Context, orgID, runID string, proof []byte) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetRunAcceptanceProof(ctx, pgstore.SetRunAcceptanceProofParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), AcceptanceProof: proof,
		})
	})
}

// LatestForProject returns the newest development run of a project.
func (s *RunStore) LatestForProject(ctx context.Context, orgID, projectID string) (domain.Run, error) {
	var out domain.Run
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).LatestRunForProject(ctx, pgstore.LatestRunForProjectParams{
			OrgID: toUUID(orgID), ProjectID: toUUID(projectID),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = runFromRow(row)
		return nil
	})
	return out, err
}

// SetTraceID records the OpenTelemetry trace for the run.
func (s *RunStore) SetTraceID(ctx context.Context, orgID, runID, traceID string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).SetRunTraceID(ctx, pgstore.SetRunTraceIDParams{
			ID: toUUID(runID), OrgID: toUUID(orgID), TraceID: traceID,
		})
	})
}

// AddApprovalVote records one distinct approver decision.
func (s *RunStore) AddApprovalVote(ctx context.Context, orgID, approvalID, userID, decision, reason string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return s.q.WithTx(tx).UpsertApprovalVote(ctx, pgstore.UpsertApprovalVoteParams{
			OrgID: toUUID(orgID), ApprovalID: toUUID(approvalID), UserID: toUUID(userID),
			Decision: decision, Reason: reason,
		})
	})
}

// CountApprovalVotes counts distinct approvers for one approval.
func (s *RunStore) CountApprovalVotes(ctx context.Context, orgID, approvalID string) (int, error) {
	var count int64
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		n, err := s.q.WithTx(tx).CountApprovalVotes(ctx, toUUID(approvalID))
		count = n
		return err
	})
	return int(count), err
}

// optionalString maps a byte slice to a nullable text column.
func optionalString(b []byte) *string {
	if len(b) == 0 {
		return nil
	}
	v := string(b)
	return &v
}
