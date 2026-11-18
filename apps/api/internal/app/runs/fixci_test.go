package runs

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeChecks struct{ items []checks.Check }

func (f fakeChecks) ListForPullRequest(context.Context, string, string) ([]checks.Check, error) {
	return f.items, nil
}

type fakePRs struct{ pr domain.PullRequest }

func (f fakePRs) PullRequest(context.Context, string, string) (domain.PullRequest, error) {
	return f.pr, nil
}

type fakeLogs struct{ body string }

func (f fakeLogs) Fetch(context.Context, string, string, string, string) (string, error) {
	return f.body, nil
}

func TestFixCICreatesADevelopmentRunWithTheCheckLog(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	plan := []byte(`{"steps":["fix the build"]}`)
	dev := domain.Run{
		ID: "dev-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "development", Status: domain.RunSucceeded, Plan: plan, Model: "m",
	}
	store.latestDev = &dev
	task := readyTask()
	task.Status = domain.TaskReview
	tasks := &retryTasks{task: task}
	enq := &fakeEnqueuer{}
	svc := newRetryService(t, store, tasks, enq).
		WithCheckList(fakeChecks{items: []checks.Check{
			{ID: "c1", Name: "ci", Status: "failed", URL: "https://github.com/o/r/actions/runs/1/job/2"},
		}}).
		WithPullRequests(fakePRs{pr: domain.PullRequest{
			ID: "pr-1", OrgID: "o", RepoID: "r-1", TaskID: "task-1", Number: 7,
		}}).
		WithCheckLogs(fakeLogs{body: "--- FAIL: TestX"})

	run, err := svc.FixCI(context.Background(), "o", "pr-1", "c1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if run.AgentType != "development" || run.Status != domain.RunQueued || run.FixOfRunID != "dev-1" || run.FixKind != "ci" {
		t.Fatalf("run = %+v, want a queued ci fix run for dev-1", run)
	}
	if string(run.FixContext) != "--- FAIL: TestX" {
		t.Fatalf("fix context = %q", run.FixContext)
	}
	if string(store.plans[run.ID]) != string(plan) {
		t.Fatalf("plan = %q, want the approved plan", store.plans[run.ID])
	}
	if tasks.task.Status != domain.TaskInProgress {
		t.Fatalf("task = %s, want in_progress", tasks.task.Status)
	}
	if len(enq.enqueued) != 1 || enq.enqueued[0] != run.ID {
		t.Fatalf("enqueued = %v", enq.enqueued)
	}
	found := false
	for _, event := range store.events {
		if event.Type == "run.fix_requested" {
			found = true
		}
	}
	if !found {
		t.Fatalf("events = %+v, want run.fix_requested", store.events)
	}
}

func TestFixCIRefusesAPassedCheck(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{}).
		WithCheckList(fakeChecks{items: []checks.Check{{ID: "c1", Status: "passed"}}}).
		WithPullRequests(fakePRs{pr: domain.PullRequest{ID: "pr-1", TaskID: "task-1"}})

	_, err := svc.FixCI(context.Background(), "o", "pr-1", "c1", "u1")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestFixCIRefusesAnUnknownCheck(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{}).
		WithCheckList(fakeChecks{}).
		WithPullRequests(fakePRs{pr: domain.PullRequest{ID: "pr-1", TaskID: "task-1"}})

	_, err := svc.FixCI(context.Background(), "o", "pr-1", "missing", "u1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
