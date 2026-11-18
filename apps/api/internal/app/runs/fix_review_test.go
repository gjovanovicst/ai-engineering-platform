package runs

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

func reviewRun(review string) domain.Run {
	return domain.Run{
		ID: "review-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "review", Status: domain.RunSucceeded, Review: []byte(review),
	}
}

func TestFixReviewCreatesADevelopmentRunOnTheSameBranch(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.runs["review-1"] = reviewRun(`{"findings":[{"severity":"blocker","body":"fix the limiter"}]}`)
	plan := []byte(`{"steps":["a"]}`)
	dev := domain.Run{
		ID: "dev-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "development", Status: domain.RunSucceeded, Plan: plan, Model: "m",
	}
	store.latestDev = &dev
	task := readyTask()
	task.Status = domain.TaskReview
	tasks := &retryTasks{task: task}
	enq := &fakeEnqueuer{}
	svc := newRetryService(t, store, tasks, enq)

	run, err := svc.FixReview(context.Background(), FixReviewInput{OrgID: "o", RunID: "review-1", Actor: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if run.AgentType != "development" || run.Status != domain.RunQueued || run.FixOfRunID != "dev-1" {
		t.Fatalf("run = %+v, want a queued development fix run for dev-1", run)
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

func TestFixReviewRefusesAReviewWithoutBlockers(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.runs["review-1"] = reviewRun(`{"findings":[{"severity":"minor","body":"nit"}]}`)
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{})

	_, err := svc.FixReview(context.Background(), FixReviewInput{OrgID: "o", RunID: "review-1"})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestFixReviewRefusesANonReviewRun(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	store.runs["review-1"] = domain.Run{
		ID: "review-1", OrgID: "o", ProjectID: "p", TaskID: "task-1",
		AgentType: "development", Status: domain.RunSucceeded,
	}
	svc := newRetryService(t, store, &retryTasks{task: readyTask()}, &fakeEnqueuer{})

	_, err := svc.FixReview(context.Background(), FixReviewInput{OrgID: "o", RunID: "review-1"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestReviewBlockers(t *testing.T) {
	t.Parallel()
	if n := reviewBlockers([]byte(`{"findings":[{"severity":"blocker"},{"severity":"minor"}]}`)); n != 1 {
		t.Fatalf("blockers = %d, want 1", n)
	}
	if n := reviewBlockers([]byte(`{"findings":[{"severity":"nit"}]}`)); n != 0 {
		t.Fatalf("blockers = %d, want 0", n)
	}
	if n := reviewBlockers(nil); n != 0 {
		t.Fatalf("blockers = %d, want 0", n)
	}
}
