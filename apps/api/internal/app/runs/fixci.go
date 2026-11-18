package runs

import (
	"context"
	"fmt"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// CheckReader lists the CI checks of a pull request.
type CheckReader interface {
	ListForPullRequest(ctx context.Context, orgID, prID string) ([]checks.Check, error)
}

// PullRequestReader loads a recorded pull request.
type PullRequestReader interface {
	PullRequest(ctx context.Context, orgID, id string) (domain.PullRequest, error)
}

// CheckLogFetcher downloads a CI check log on the host (the sandbox stays
// offline). It is implemented by adapters/git for GitHub Actions.
type CheckLogFetcher interface {
	Fetch(ctx context.Context, orgID, projectID, repoID, checkURL string) (string, error)
}

// fixContextLimit bounds the log kept in the run; the tail carries the failure.
const fixContextLimit = 200_000

// WithCheckList attaches the CI check reader used by FixCI.
func (s *Service) WithCheckList(reader CheckReader) *Service {
	s.checkList = reader
	return s
}

// WithPullRequests attaches the pull-request reader used by FixCI.
func (s *Service) WithPullRequests(reader PullRequestReader) *Service {
	s.pullRequests = reader
	return s
}

// WithCheckLogs attaches the CI log fetcher used by FixCI.
func (s *Service) WithCheckLogs(fetcher CheckLogFetcher) *Service {
	s.checkLogs = fetcher
	return s
}

// FixCI starts a development run that fixes a failed CI check on the pull
// request: the same approved plan and branch, with the check log in the prompt.
func (s *Service) FixCI(ctx context.Context, orgID, prID, checkID, actor string) (domain.Run, error) {
	if s.checkList == nil || s.pullRequests == nil {
		return domain.Run{}, fmt.Errorf("runs: the CI check fix is not configured")
	}
	pr, err := s.pullRequests.PullRequest(ctx, orgID, prID)
	if err != nil {
		return domain.Run{}, err
	}
	list, err := s.checkList.ListForPullRequest(ctx, orgID, prID)
	if err != nil {
		return domain.Run{}, err
	}
	var check checks.Check
	found := false
	for _, c := range list {
		if c.ID == checkID {
			check, found = c, true
			break
		}
	}
	if !found {
		return domain.Run{}, fmt.Errorf("%w: the check does not belong to the pull request", domain.ErrNotFound)
	}
	if check.Status != "failed" {
		return domain.Run{}, fmt.Errorf("%w: only a failed check can be fixed", domain.ErrValidation)
	}
	dev, err := s.store.LatestRun(ctx, orgID, pr.TaskID, string(domain.AgentDevelopment))
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
	task, err := s.tasks.Get(ctx, orgID, dev.ProjectID, pr.TaskID)
	if err != nil {
		return domain.Run{}, err
	}
	if task.Status.Terminal() {
		return domain.Run{}, &domain.TransitionError{
			From: task.Status, To: domain.TaskInProgress,
			Detail: "the task is already closed (the pull request may be merged)",
		}
	}
	logText := ""
	if s.checkLogs != nil {
		logText, _ = s.checkLogs.Fetch(ctx, orgID, dev.ProjectID, pr.RepoID, check.URL)
	}
	if strings.TrimSpace(logText) == "" {
		logText = check.Output
	}
	if logText == "" {
		logText = check.Name + ": no log was available; read the check output."
	}
	if len(logText) > fixContextLimit {
		logText = "[truncated]\n" + logText[len(logText)-fixContextLimit:]
	}
	if err := retryTask(ctx, &task); err != nil {
		return domain.Run{}, err
	}
	created, err := s.store.Create(ctx, domain.Run{
		OrgID: orgID, ProjectID: dev.ProjectID, TaskID: pr.TaskID,
		AgentType: string(domain.AgentDevelopment), Status: domain.RunQueued,
		Model: dev.Model, Budget: dev.Budget, FixOfRunID: dev.ID,
		FixKind: "ci", FixContext: []byte(logText),
		FrozenContextHash: dev.FrozenContextHash, FrozenContextSources: dev.FrozenContextSources,
	})
	if err != nil {
		return domain.Run{}, err
	}
	if err := s.store.SetPlan(ctx, orgID, created.ID, dev.Plan, ""); err != nil {
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
	_ = s.store.AppendEvent(ctx, orgID, created.ID, "run.fix_requested", "user",
		map[string]any{"from_run": dev.ID, "kind": "ci", "check": check.Name, "actor": actor})
	return created, nil
}
