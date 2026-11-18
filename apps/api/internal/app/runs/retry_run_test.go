package runs

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type retryTasks struct{ task domain.Task }

func (f *retryTasks) Get(context.Context, string, string, string) (domain.Task, error) {
	return f.task, nil
}

func (f *retryTasks) UpdateStatus(_ context.Context, t domain.Task) (domain.Task, error) {
	f.task = t
	return t, nil
}

func newRetryService(t *testing.T, store *fakeStore, tasks *retryTasks, enq *fakeEnqueuer) *Service {
	t.Helper()
	svc, err := New(store, tasks, &fakeExecutor{}, enq, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRetryCreatesADevelopmentRunWithTheSamePlan(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	plan := []byte(`{"steps":["write the code"],"files":["main.go"],"risks":[]}`)
	store.runs["run-1"] = domain.Run{
		ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "development", Status: domain.RunFailed, Plan: plan, Model: "m",
	}
	task := readyTask()
	task.Status = domain.TaskBlocked
	tasks := &retryTasks{task: task}
	enq := &fakeEnqueuer{}
	svc := newRetryService(t, store, tasks, enq)

	run, err := svc.Retry(context.Background(), RetryInput{OrgID: "o", RunID: "run-1", Actor: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if run.AgentType != "development" || run.Status != domain.RunQueued {
		t.Fatalf("run = %+v, want a queued development run", run)
	}
	if string(store.plans[run.ID]) != string(plan) {
		t.Fatalf("plan = %q, want the approved plan", store.plans[run.ID])
	}
	if tasks.task.Status != domain.TaskInProgress {
		t.Fatalf("task status = %s, want in_progress", tasks.task.Status)
	}
	if len(enq.enqueued) != 1 || enq.enqueued[0] != run.ID {
		t.Fatalf("enqueued = %v, want the new run", enq.enqueued)
	}
	found := false
	for _, event := range store.events {
		if event.Type == "run.retried" {
			found = true
		}
	}
	if !found {
		t.Fatalf("events = %+v, want run.retried", store.events)
	}
}

func TestRetryRefusesARunWithoutAPlan(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "task-1", AgentType: "development", Status: domain.RunFailed}
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{})

	_, err := svc.Retry(context.Background(), RetryInput{OrgID: "o", RunID: "run-1"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestRetryRefusesASucceededRun(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{
		ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "development", Status: domain.RunSucceeded, Plan: []byte(`{}`),
	}
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{})

	_, err := svc.Retry(context.Background(), RetryInput{OrgID: "o", RunID: "run-1"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}
