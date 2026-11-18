// Package runs owns the run lifecycle: start, cancel, read.
package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/mergequeue"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/policyschedule"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Store is the persistence port.
type Store interface {
	Create(ctx context.Context, r domain.Run) (domain.Run, error)
	Get(ctx context.Context, orgID, id string) (domain.Run, error)
	UpdateStatus(ctx context.Context, r domain.Run) (domain.Run, error)
	AppendEvent(ctx context.Context, orgID, runID, eventType, actor string, payload map[string]any) error
	Events(ctx context.Context, orgID, runID string) ([]Event, error)
	EventsAfter(ctx context.Context, orgID, runID string, after int64) ([]Event, error)
	SetPlan(ctx context.Context, orgID, runID string, plan []byte, sha string) error
	SetDiff(ctx context.Context, orgID, runID string, diff []byte, sha string) error
	SetReview(ctx context.Context, orgID, runID string, review []byte) error
	SetAcceptanceProof(ctx context.Context, orgID, runID string, proof []byte) error
	SetContextHash(ctx context.Context, orgID, runID, hash string) error
	SetTraceID(ctx context.Context, orgID, runID, traceID string) error
	AddApprovalVote(ctx context.Context, orgID, approvalID, userID, decision, reason string) error
	CountApprovalVotes(ctx context.Context, orgID, approvalID string) (int, error)
	LatestReviewForTask(ctx context.Context, orgID, taskID string) ([]byte, error)
	LatestRun(ctx context.Context, orgID, taskID, agentType string) (domain.Run, error)
	ListForTask(ctx context.Context, orgID, taskID string, limit int) ([]domain.Run, error)
	LatestForProject(ctx context.Context, orgID, projectID string) (domain.Run, error)
	CreateApproval(ctx context.Context, orgID, runID, pullRequestID, kind string, expires time.Time) (Approval, error)
	GetApproval(ctx context.Context, orgID, id string) (Approval, error)
	DecideApproval(ctx context.Context, orgID, id, status, reason, userID string, at time.Time) (Approval, error)
	PendingApprovals(ctx context.Context, orgID string, limit int) ([]Approval, error)
	CountActive(ctx context.Context, orgID string) (int, error)
	ExpireApprovals(ctx context.Context, orgID string) ([]ExpiredApproval, error)
}

// Event is one entry of the run log.
type Event struct {
	Seq       int64
	Type      string
	Actor     string
	Payload   map[string]any
	CreatedAt time.Time
}

// TaskReader loads the task a run executes; TaskWriter persists its status.
type TaskReader interface {
	Get(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
	UpdateStatus(ctx context.Context, t domain.Task) (domain.Task, error)
}

// Approval is a pending human decision.
type Approval struct {
	ID            string
	OrgID         string
	RunID         string
	Type          string
	Status        string
	RequestedAt   time.Time
	DecidedAt     *time.Time
	DecidedBy     string
	Reason        string
	PullRequestID string
	Number        int
	URL           string
}

// Enqueuer schedules a run for the worker. When nil, the service executes the
// run in-process (tests and single-binary development).
type Enqueuer interface {
	Enqueue(ctx context.Context, runID, orgID string) error
}

// CheckGate reports whether CI checks block a merge.
type CheckGate interface {
	Blocked(ctx context.Context, orgID, prID string) (bool, string, error)
}

// Merger merges an approved PR/MR (implemented in adapters/git).
type Merger interface {
	Merge(ctx context.Context, orgID, projectID string, number int) error
}

// Executor performs the work of a run (implemented by the agent runner).
type Executor interface {
	Execute(ctx context.Context, run domain.Run, prompt string, contextRules []string, designRef string, forbiddenImports []string, criteria []string, sources []ContextSource, e2eCommand string, visualCommand string, mcpConfig string, testCommand string, visualThreshold float64, visualMasks []visualdiff.Rect, policyMaxDiff int, forbiddenPaths []string, allowedModels []string, allowedTools []string, pluginChecks []PluginCheck) (domain.Run, error)
}

// Service coordinates runs.
type Service struct {
	store         Store
	tasks         TaskReader
	executor      Executor
	enqueuer      Enqueuer
	merger        Merger
	mergeQueue    *mergequeue.Service
	windows       ChangeWindowReader
	checks        CheckGate
	checkList     CheckReader
	pullRequests  PullRequestReader
	checkLogs     CheckLogFetcher
	maxActiveRuns int
	log           *slog.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// WithMaxActiveRuns caps concurrent queued/running runs per org (0 = no cap).
// ChangeWindowReader returns the project merge window JSON.
type ChangeWindowReader interface {
	ChangeWindow(ctx context.Context, orgID, projectID string) (string, error)
	RequiredApprovals(ctx context.Context, orgID, projectID string) (int, error)
}

// WithChangeWindow enforces the project merge window at approval time.
func (s *Service) WithChangeWindow(reader ChangeWindowReader) *Service {
	s.windows = reader
	return s
}

// WithMergeQueue serialises approved merges per project.
func (s *Service) WithMergeQueue(queue *mergequeue.Service) *Service {
	s.mergeQueue = queue
	return s
}

func (s *Service) WithMaxActiveRuns(n int) *Service {
	s.maxActiveRuns = n
	return s
}

// WithChecks attaches the CI merge gate.
func (s *Service) WithChecks(gate CheckGate) *Service {
	s.checks = gate
	return s
}

// New wires the service.
func New(store Store, tasks TaskReader, executor Executor, enqueuer Enqueuer, merger Merger, log *slog.Logger) (*Service, error) {
	if store == nil || tasks == nil || executor == nil {
		return nil, errors.New("runs: store, tasks and executor are required")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, tasks: tasks, executor: executor, enqueuer: enqueuer, merger: merger, log: log, running: map[string]context.CancelFunc{}}, nil
}

// StartInput is the start-run payload.
type StartInput struct {
	OrgID     string
	ProjectID string
	TaskID    string
	AgentType string
	Model     string
	Budget    domain.Budget
}

// Start validates the task, creates the run and executes it in the background.
// The queue (River) replaces the goroutine in E3-S2.
func (s *Service) Start(ctx context.Context, in StartInput) (domain.Run, error) {
	agentType, err := domain.ParseAgentType(in.AgentType)
	if err != nil {
		return domain.Run{}, err
	}
	if s.maxActiveRuns > 0 {
		if active, cerr := s.store.CountActive(ctx, in.OrgID); cerr == nil && active >= s.maxActiveRuns {
			return domain.Run{}, domain.ErrRateLimited
		}
	}
	task, err := s.tasks.Get(ctx, in.OrgID, in.ProjectID, in.TaskID)
	if err != nil {
		return domain.Run{}, err
	}
	if deps, ok := s.tasks.(interface {
		DependenciesBlocked(ctx context.Context, orgID, projectID, taskID string) (string, error)
	}); ok {
		if blockedBy, derr := deps.DependenciesBlocked(ctx, in.OrgID, in.ProjectID, task.ID); derr != nil {
			return domain.Run{}, derr
		} else if blockedBy != "" {
			return domain.Run{}, fmt.Errorf("%w: blocked by dependency %s", domain.ErrValidation, blockedBy)
		}
	}
	if task.Status != domain.TaskReady {
		return domain.Run{}, &domain.TransitionError{
			From: task.Status, To: domain.TaskPlanning,
			Detail: "only tasks in ready can start a run",
		}
	}

	run, err := s.store.Create(ctx, domain.Run{
		OrgID: in.OrgID, ProjectID: in.ProjectID, TaskID: task.ID,
		AgentType: string(agentType), Status: domain.RunQueued, Model: in.Model, Budget: in.Budget,
	})
	if err != nil {
		return domain.Run{}, err
	}

	// ASDLC §2: a planning run moves the task to planning.
	if agentType == domain.AgentPlanning {
		if err := task.Transition(domain.TaskPlanning, domain.TransitionOptions{Actor: domain.ActorHuman}); err == nil {
			_, _ = s.tasks.UpdateStatus(ctx, task)
		}
	}

	if s.enqueuer != nil {
		if err := s.enqueuer.Enqueue(ctx, run.ID, run.OrgID); err != nil {
			return domain.Run{}, err
		}
		return run, nil
	}

	prompt := BuildPrompt(task)
	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[run.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, run.ID)
			s.mu.Unlock()
			cancel()
		}()
		_, _ = s.executor.Execute(runCtx, run, prompt, nil, "", nil, nil, nil, "", "", "", "", 0.01, nil, 0, nil, nil, nil, nil)
	}()

	return run, nil
}

// Get returns one run.
func (s *Service) Get(ctx context.Context, orgID, id string) (domain.Run, error) {
	return s.store.Get(ctx, orgID, id)
}

// Events returns the run log.
func (s *Service) Events(ctx context.Context, orgID, id string) ([]Event, error) {
	return s.store.Events(ctx, orgID, id)
}

// ListForTask returns the task's runs.
// LatestForProject returns the newest development run of a project.
func (s *Service) LatestForProject(ctx context.Context, orgID, projectID string) (domain.Run, error) {
	return s.store.LatestForProject(ctx, orgID, projectID)
}

func (s *Service) ListForTask(ctx context.Context, orgID, taskID string, limit int) ([]domain.Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.store.ListForTask(ctx, orgID, taskID, limit)
}

// ExpiredApproval identifies an approval that passed its deadline.
type ExpiredApproval struct {
	ID    string
	RunID string
	Type  string
}

// SweepExpired marks pending approvals past their deadline as expired and
// records an event on each affected run.
func (s *Service) SweepExpired(ctx context.Context, orgID string) error {
	expired, err := s.store.ExpireApprovals(ctx, orgID)
	if err != nil {
		return err
	}
	for _, approval := range expired {
		_ = s.store.AppendEvent(ctx, orgID, approval.RunID, "approval.expired", "system",
			map[string]any{"approval_id": approval.ID, "type": approval.Type})
	}
	return nil
}

// PendingApprovals lists approvals waiting for a human.
func (s *Service) PendingApprovals(ctx context.Context, orgID string, limit int) ([]Approval, error) {
	if err := s.SweepExpired(ctx, orgID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.PendingApprovals(ctx, orgID, limit)
}

// Decide records an approval decision and applies its consequences:
// approved plan -> run succeeds and the task moves to in_progress;
// rejected plan -> run fails and the task is blocked with the reason.
func (s *Service) Decide(ctx context.Context, orgID, approvalID, decision, reason, userID string) (domain.Run, string, error) {
	if err := s.SweepExpired(ctx, orgID); err != nil {
		return domain.Run{}, "", err
	}
	approval, err := s.store.GetApproval(ctx, orgID, approvalID)
	if err != nil {
		return domain.Run{}, "", err
	}
	if approval.Status != "pending" {
		return domain.Run{}, "", &domain.TransitionError{From: domain.TaskStatus(approval.Status), To: domain.TaskStatus(approval.Status), Detail: "the approval was already decided"}
	}
	run, err := s.store.Get(ctx, orgID, approval.RunID)
	if err != nil {
		return domain.Run{}, "", err
	}

	now := time.Now().UTC()

	// A PR/MR approval only unlocks the merge; the task moves on the webhook.
	if approval.Type == "pr" {
		status := "rejected"
		if decision == "approved" {
			status = "approved"
		} else if decision != "rejected" {
			return domain.Run{}, "", &domain.TransitionError{Detail: "decision must be approved or rejected"}
		}
		manual := manualEvidence(run.AcceptanceProof)
		if status == "approved" && len(manual) > 0 && strings.TrimSpace(reason) == "" {
			return domain.Run{}, "", fmt.Errorf("%w: the acceptance proof contains manual evidence; a decision note is required", domain.ErrValidation)
		}
		if status == "approved" && approval.PullRequestID != "" {
			if s.checks != nil {
				blocked, why, ferr := s.checks.Blocked(ctx, orgID, approval.PullRequestID)
				if ferr != nil {
					s.log.Warn("runs: check gate", slog.Any("err", ferr))
				} else if blocked {
					_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.blocked", "system",
						map[string]any{"reason": why})
					return domain.Run{}, "", fmt.Errorf("%w: %s", domain.ErrValidation, why)
				}
			}
			if review, rerr := s.store.LatestReviewForTask(ctx, orgID, run.TaskID); rerr == nil && len(review) > 0 {
				if why := blockerReason(review); why != "" {
					_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.blocked", "system",
						map[string]any{"reason": why})
					return domain.Run{}, "", fmt.Errorf("%w: %s", domain.ErrValidation, why)
				}
			}
		}
		if _, err := s.store.DecideApproval(ctx, orgID, approvalID, status, reason, userID, now); err != nil {
			return domain.Run{}, "", err
		}
		if status == "approved" && len(manual) > 0 {
			_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "acceptance.manual_ack", "user",
				map[string]any{"reviewer": userID, "reason": reason, "criteria": manual})
		}
		if status == "approved" && s.windows != nil {
			if raw, werr := s.windows.ChangeWindow(ctx, run.OrgID, run.ProjectID); werr == nil && raw != "" {
				if window, perr := policyschedule.Parse(raw); perr == nil && !window.Allows(time.Now()) {
					_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.blocked_window", "system", map[string]any{"window": raw})
					return domain.Run{}, "", fmt.Errorf("%w: outside the project change window", domain.ErrValidation)
				}
			}
		}
		required := 1
		if s.windows != nil {
			if n, nerr := s.windows.RequiredApprovals(ctx, run.OrgID, run.ProjectID); nerr == nil && n > 0 {
				required = n
			}
		}
		if status == "approved" && required > 1 {
			if verr := s.store.AddApprovalVote(ctx, run.OrgID, approvalID, userID, "approved", reason); verr != nil {
				return domain.Run{}, "", verr
			}
			votes, verr := s.store.CountApprovalVotes(ctx, run.OrgID, approvalID)
			if verr != nil {
				return domain.Run{}, "", verr
			}
			if votes < required {
				_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "approval.vote", "user",
					map[string]any{"approver": userID, "votes": votes, "required": required})
				return run, "", fmt.Errorf("%w: approval recorded; %d more required", domain.ErrValidation, required-votes)
			}
		}
		if status == "approved" && s.mergeQueue != nil && approval.Number > 0 {
			_, qerr := s.mergeQueue.Enqueue(ctx, mergequeue.Item{
				OrgID: run.OrgID, ProjectID: run.ProjectID, RunID: run.ID, TaskID: run.TaskID, PullRequestID: approval.PullRequestID, Number: approval.Number,
			})
			if qerr != nil {
				s.log.Warn("runs: merge queue enqueue", slog.Any("err", qerr))
				_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.failed", "system", map[string]any{"error": qerr.Error()})
			} else {
				_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.queued", "system", map[string]any{"number": approval.Number})
				results, perr := s.mergeQueue.ProcessAll(ctx, run.OrgID, run.ProjectID)
				if perr != nil {
					s.log.Warn("runs: merge queue", slog.Any("err", perr))
					_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.failed", "system", map[string]any{"error": perr.Error()})
				}
				for _, result := range results {
					if !result.Merged {
						_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.failed", "system",
							map[string]any{"number": result.Item.Number, "error": result.Err.Error()})
						continue
					}
					_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.requested", "system",
						map[string]any{"number": result.Item.Number})
					if merger, ok := s.tasks.(interface {
						MarkMerged(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
					}); ok && result.Item.TaskID != "" {
						if _, terr := merger.MarkMerged(ctx, run.OrgID, run.ProjectID, result.Item.TaskID); terr != nil {
							s.log.Warn("runs: mark merged", slog.Any("err", terr))
						}
					}
				}
			}
		} else if status == "approved" && s.merger != nil && approval.Number > 0 {
			if merr := s.merger.Merge(ctx, run.OrgID, run.ProjectID, approval.Number); merr != nil {
				s.log.Warn("runs: auto-merge", slog.Any("err", merr))
				_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.failed", "system",
					map[string]any{"error": merr.Error()})
			} else {
				_ = s.store.AppendEvent(ctx, run.OrgID, run.ID, "merge.requested", "system",
					map[string]any{"number": approval.Number})
				// Close the task as soon as the merge API call succeeded; the
				// webhook is a backup, not the only path.
				if merger, ok := s.tasks.(interface {
					MarkMerged(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
				}); ok {
					if _, terr := merger.MarkMerged(ctx, run.OrgID, run.ProjectID, run.TaskID); terr != nil {
						s.log.Warn("runs: mark merged", slog.Any("err", terr))
					}
				}
			}
		}
		return run, "", nil
	}

	switch decision {
	case "approved":
		if _, err := s.store.DecideApproval(ctx, orgID, approvalID, "approved", reason, userID, now); err != nil {
			return domain.Run{}, "", err
		}
		if err := run.Transition(domain.RunRunning, now); err != nil {
			return domain.Run{}, "", err
		}
		if _, err := s.store.UpdateStatus(ctx, run); err != nil {
			return domain.Run{}, "", err
		}
		if err := run.Transition(domain.RunSucceeded, now); err != nil {
			return domain.Run{}, "", err
		}
		updated, err := s.store.UpdateStatus(ctx, run)
		if err != nil {
			return domain.Run{}, "", err
		}
		developmentRunID := ""
		if task, terr := s.tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID); terr == nil {
			if terr := task.Transition(domain.TaskInProgress, domain.TransitionOptions{
				Actor: domain.ActorHuman, Reason: reason, PlanApproved: true,
			}); terr == nil {
				_, _ = s.tasks.UpdateStatus(ctx, task)
			}
		}
		frozenSources := ""
		if events, eerr := s.store.Events(ctx, run.OrgID, run.ID); eerr == nil {
			for _, event := range events {
				if event.Type != "context.manifest" {
					continue
				}
				if raw, ok := event.Payload["sources"]; ok {
					if encoded, merr := json.Marshal(raw); merr == nil {
						frozenSources = string(encoded)
					}
				}
			}
		}
		// The approved plan starts the implementation run.
		if s.enqueuer != nil {
			devRun, derr := s.store.Create(ctx, domain.Run{
				OrgID: run.OrgID, ProjectID: run.ProjectID, TaskID: run.TaskID,
				AgentType: string(domain.AgentDevelopment), Status: domain.RunQueued, Model: run.Model,
				Budget:            run.Budget, // the approved plan's budget covers its implementation
				FrozenContextHash: run.ContextHash, FrozenContextSources: frozenSources,
			})
			if derr == nil {
				developmentRunID = devRun.ID
				// The development prompt includes run.Plan; without this copy the
				// approved plan never reaches the implementation agent.
				if len(run.Plan) > 0 {
					if perr := s.store.SetPlan(ctx, run.OrgID, devRun.ID, run.Plan, ""); perr != nil {
						s.log.Warn("runs: copy the approved plan to the development run", slog.Any("err", perr))
					}
				}
				if eerr := s.enqueuer.Enqueue(ctx, devRun.ID, devRun.OrgID); eerr != nil {
					s.log.Error("runs: enqueue development", slog.Any("err", eerr))
				}
			}
		}
		return updated, developmentRunID, nil

	case "rejected":
		if _, err := s.store.DecideApproval(ctx, orgID, approvalID, "rejected", reason, userID, now); err != nil {
			return domain.Run{}, "", err
		}
		run.FailureCode = "plan_rejected"
		run.FailureDetail = reason
		if err := run.Transition(domain.RunFailed, now); err != nil {
			return domain.Run{}, "", err
		}
		updated, err := s.store.UpdateStatus(ctx, run)
		if err != nil {
			return domain.Run{}, "", err
		}
		if task, terr := s.tasks.Get(ctx, run.OrgID, run.ProjectID, run.TaskID); terr == nil {
			if terr := task.Transition(domain.TaskBlocked, domain.TransitionOptions{
				Actor: domain.ActorHuman, Reason: "plan rejected: " + reason,
			}); terr == nil {
				_, _ = s.tasks.UpdateStatus(ctx, task)
			}
		}
		return updated, "", nil

	default:
		return domain.Run{}, "", &domain.TransitionError{Detail: "decision must be approved or rejected"}
	}
}

// RetryInput identifies a failed development run to retry.
type RetryInput struct {
	OrgID string
	RunID string
	Actor string
}

// Retry starts a new development run with the failed run's approved plan, so a
// failure does not need a new planning run and a new plan approval.
func (s *Service) Retry(ctx context.Context, in RetryInput) (domain.Run, error) {
	run, err := s.store.Get(ctx, in.OrgID, in.RunID)
	if err != nil {
		return domain.Run{}, err
	}
	if run.AgentType != string(domain.AgentDevelopment) {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(run.AgentType), To: domain.TaskStatus(domain.RunQueued),
			Detail: "only development runs can be retried",
		}
	}
	if run.Status != domain.RunFailed {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(run.Status), To: domain.TaskStatus(domain.RunQueued),
			Detail: "only a failed run can be retried",
		}
	}
	if len(run.Plan) == 0 {
		return domain.Run{}, fmt.Errorf("%w: the failed run has no approved plan to retry", domain.ErrValidation)
	}
	if latest, lerr := s.store.LatestRun(ctx, in.OrgID, run.TaskID, string(domain.AgentDevelopment)); lerr == nil && !latest.Status.Terminal() {
		return domain.Run{}, fmt.Errorf("%w: a run is already active for the task", domain.ErrConflict)
	}
	task, err := s.tasks.Get(ctx, in.OrgID, run.ProjectID, run.TaskID)
	if err != nil {
		return domain.Run{}, err
	}
	if err := retryTask(ctx, &task); err != nil {
		return domain.Run{}, err
	}
	created, err := s.store.Create(ctx, domain.Run{
		OrgID: run.OrgID, ProjectID: run.ProjectID, TaskID: run.TaskID,
		AgentType: string(domain.AgentDevelopment), Status: domain.RunQueued,
		Model: run.Model, Budget: run.Budget,
		FrozenContextHash: run.FrozenContextHash, FrozenContextSources: run.FrozenContextSources,
	})
	if err != nil {
		return domain.Run{}, err
	}
	if err := s.store.SetPlan(ctx, in.OrgID, created.ID, run.Plan, ""); err != nil {
		return domain.Run{}, err
	}
	if _, err := s.tasks.UpdateStatus(ctx, task); err != nil {
		return domain.Run{}, err
	}
	if s.enqueuer != nil {
		if err := s.enqueuer.Enqueue(ctx, created.ID, created.OrgID); err != nil {
			return domain.Run{}, err
		}
	}
	_ = s.store.AppendEvent(ctx, in.OrgID, created.ID, "run.retried", "user",
		map[string]any{"from_run": run.ID, "actor": in.Actor})
	return created, nil
}

// retryTask moves the task back to in_progress for a new development run. A
// blocked task goes through ready and planning (the state machine only allows
// planning -> in_progress with an approved plan); a task left in progress or
// review by an older failure is blocked first.
func retryTask(_ context.Context, task *domain.Task) error {
	opt := domain.TransitionOptions{Actor: domain.ActorHuman, Reason: "retry with the approved plan"}
	switch task.Status {
	case domain.TaskInProgress, domain.TaskReview:
		if err := task.Transition(domain.TaskBlocked, opt); err != nil {
			return err
		}
	case domain.TaskBlocked, domain.TaskReady:
		// fine as is
	default:
		return &domain.TransitionError{
			From: task.Status, To: domain.TaskInProgress,
			Detail: "the task cannot be retried from " + string(task.Status),
		}
	}
	if task.Status == domain.TaskBlocked {
		if err := task.Transition(domain.TaskReady, opt); err != nil {
			return err
		}
	}
	if err := task.Transition(domain.TaskPlanning, opt); err != nil {
		return err
	}
	return task.Transition(domain.TaskInProgress, domain.TransitionOptions{
		Actor: domain.ActorHuman, Reason: "retry with the approved plan", PlanApproved: true,
	})
}

// FixReviewInput identifies the review run whose blockers should be fixed.
type FixReviewInput struct {
	OrgID string
	RunID string
	Actor string
}

// FixReview starts a development run that fixes the blockers of a review on
// the existing pull request: the same approved plan, the same branch (the
// development run's), and the review findings in the prompt.
func (s *Service) FixReview(ctx context.Context, in FixReviewInput) (domain.Run, error) {
	review, err := s.store.Get(ctx, in.OrgID, in.RunID)
	if err != nil {
		return domain.Run{}, err
	}
	if review.AgentType != string(domain.AgentReview) {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(review.AgentType), To: domain.TaskStatus(domain.RunQueued),
			Detail: "only review runs can be fixed",
		}
	}
	if review.Status != domain.RunSucceeded {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(review.Status), To: domain.TaskStatus(domain.RunQueued),
			Detail: "only a finished review can be fixed",
		}
	}
	if reviewBlockers(review.Review) == 0 {
		return domain.Run{}, fmt.Errorf("%w: the review has no blocker findings", domain.ErrValidation)
	}
	dev, err := s.store.LatestRun(ctx, in.OrgID, review.TaskID, string(domain.AgentDevelopment))
	if err != nil {
		return domain.Run{}, fmt.Errorf("find the development run: %w", err)
	}
	if dev.Status != domain.RunSucceeded {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(dev.Status), To: domain.TaskStatus(domain.RunQueued),
			Detail: "the pull request's development run has not finished",
		}
	}
	if len(dev.Plan) == 0 {
		return domain.Run{}, fmt.Errorf("%w: the development run has no approved plan", domain.ErrValidation)
	}
	task, err := s.tasks.Get(ctx, in.OrgID, review.ProjectID, review.TaskID)
	if err != nil {
		return domain.Run{}, err
	}
	if task.Status.Terminal() {
		return domain.Run{}, &domain.TransitionError{
			From: task.Status, To: domain.TaskInProgress,
			Detail: "the task is already closed (the pull request may be merged)",
		}
	}
	if err := retryTask(ctx, &task); err != nil {
		return domain.Run{}, err
	}
	created, err := s.store.Create(ctx, domain.Run{
		OrgID: review.OrgID, ProjectID: review.ProjectID, TaskID: review.TaskID,
		AgentType: string(domain.AgentDevelopment), Status: domain.RunQueued,
		Model: dev.Model, Budget: dev.Budget, FixOfRunID: dev.ID,
		FrozenContextHash: dev.FrozenContextHash, FrozenContextSources: dev.FrozenContextSources,
	})
	if err != nil {
		return domain.Run{}, err
	}
	if err := s.store.SetPlan(ctx, in.OrgID, created.ID, dev.Plan, ""); err != nil {
		return domain.Run{}, err
	}
	if _, err := s.tasks.UpdateStatus(ctx, task); err != nil {
		return domain.Run{}, err
	}
	if s.enqueuer != nil {
		if err := s.enqueuer.Enqueue(ctx, created.ID, created.OrgID); err != nil {
			return domain.Run{}, err
		}
	}
	_ = s.store.AppendEvent(ctx, in.OrgID, created.ID, "run.fix_requested", "user",
		map[string]any{"from_run": dev.ID, "review_run": review.ID, "actor": in.Actor})
	return created, nil
}

// fixBranch is the branch a run publishes to. A fix run updates the branch of
// the development run it fixes; every other run gets its own branch.
func fixBranch(run domain.Run) string {
	id := run.ID
	if run.FixOfRunID != "" {
		id = run.FixOfRunID
	}
	if len(id) > 8 {
		id = id[:8]
	}
	return "aip/" + id
}

// EventsAfter returns events after a sequence number (SSE replay).
func (s *Service) EventsAfter(ctx context.Context, orgID, id string, after int64) ([]Event, error) {
	return s.store.EventsAfter(ctx, orgID, id, after)
}

// Cancel requests cancellation and marks the run canceled.
func (s *Service) Cancel(ctx context.Context, orgID, id string) (domain.Run, error) {
	run, err := s.store.Get(ctx, orgID, id)
	if err != nil {
		return domain.Run{}, err
	}
	if run.Status.Terminal() {
		return domain.Run{}, &domain.TransitionError{
			From: domain.TaskStatus(run.Status), To: domain.TaskStatus(domain.RunCanceled),
			Detail: "the run already finished",
		}
	}
	now := time.Now().UTC()
	run.CancelRequestedAt = &now
	if err := run.Transition(domain.RunCanceled, now); err != nil {
		return domain.Run{}, err
	}
	updated, err := s.store.UpdateStatus(ctx, run)
	if err != nil {
		return domain.Run{}, err
	}

	s.mu.Lock()
	cancel := s.running[id]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return updated, nil
}

// BuildPrompt renders the task briefing passed to the agent.
func BuildPrompt(task domain.Task) string {
	var b strings.Builder
	b.WriteString("Task: " + task.Ref + " — " + task.Title + "\n\n")
	if task.Description != "" {
		b.WriteString("Description:\n" + task.Description + "\n\n")
	}
	if len(task.Acceptance) > 0 {
		b.WriteString("Acceptance criteria:\n")
		for i, ac := range task.Acceptance {
			fmt.Fprintf(&b, "%d. Given %s, when %s, then %s.\n", i+1, ac.Given, ac.When, ac.Then)
		}
	}
	return b.String()
}

// blockerReason returns a message when a review contains a blocker finding.
func blockerReason(raw []byte) string {
	var payload struct {
		Findings []struct {
			Severity string `json:"severity"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Message  string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, f := range payload.Findings {
		if f.Severity == "blocker" {
			where := f.File
			if f.Line > 0 {
				where = fmt.Sprintf("%s:%d", where, f.Line)
			}
			return fmt.Sprintf("review blocker at %s: %s", where, f.Message)
		}
	}
	return ""
}

// DesignPrompt renders the compact standards block injected into the prompt.
func DesignPrompt(name string, version int, sha string, raw []byte) string {
	var spec struct {
		Tokens        []struct{ Name, Value, Category string } `json:"tokens"`
		Accessibility []string                                 `json:"accessibility"`
		Responsive    []string                                 `json:"responsive"`
		Patterns      []string                                 `json:"patterns"`
	}
	_ = json.Unmarshal(raw, &spec)
	var b strings.Builder
	fmt.Fprintf(&b, "DESIGN STANDARD: %s v%d (sha256 %s)\n", name, version, sha)
	if len(spec.Tokens) > 0 {
		b.WriteString("Tokens (use these values instead of raw values):\n")
		for _, token := range spec.Tokens {
			fmt.Fprintf(&b, "- %s = %s (%s)\n", token.Name, token.Value, token.Category)
		}
	}
	writeList := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s:\n", label)
		for _, item := range items {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	writeList("Accessibility", spec.Accessibility)
	writeList("Responsive", spec.Responsive)
	writeList("Patterns", spec.Patterns)
	b.WriteString("Use only approved components and design tokens; do not introduce new colours, spacing values or UI libraries.\n")
	out := b.String()
	if len(out) > 4000 {
		out = out[:4000] + "\n[standards truncated]\n"
	}
	return out
}

// manualEvidence lists criteria whose proof is manual.
func manualEvidence(proof []byte) []string {
	if len(proof) == 0 {
		return nil
	}
	var parsed struct {
		Evidence []struct {
			Criterion string `json:"criterion"`
			Kind      string `json:"kind"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(proof, &parsed); err != nil {
		return nil
	}
	var out []string
	for _, item := range parsed.Evidence {
		if item.Kind == "manual" {
			out = append(out, item.Criterion)
		}
	}
	return out
}
