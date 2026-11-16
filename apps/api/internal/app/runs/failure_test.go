package runs

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type blockingTasks struct{ task domain.Task }

func (b *blockingTasks) Get(context.Context, string, string, string) (domain.Task, error) {
	return b.task, nil
}

func (b *blockingTasks) UpdateStatus(_ context.Context, t domain.Task) (domain.Task, error) {
	b.task = t
	return t, nil
}

func TestBlockTaskAfterFailureParksTheTask(t *testing.T) {
	t.Parallel()
	task := readyTask()
	task.Status = domain.TaskInProgress
	tasks := &blockingTasks{task: task}
	store := newFakeStore()
	worker := NewJobWorker(store, tasks, nil, nil, slog.New(slog.DiscardHandler))

	worker.blockTaskAfterFailure(context.Background(), domain.Run{
		ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: task.ID,
		Status: domain.RunFailed, FailureCode: "test_gate_failed", FailureDetail: "gofmt: file.go",
	})

	if tasks.task.Status != domain.TaskBlocked {
		t.Fatalf("task status = %s, want blocked", tasks.task.Status)
	}
	if tasks.task.BlockedReason == "" {
		t.Fatal("a blocked task needs a reason")
	}
	if len(store.events) != 1 || store.events[0].Type != "task.blocked" {
		t.Fatalf("events = %+v, want one task.blocked event", store.events)
	}
}

func TestBlockTaskAfterFailureSkipsTerminalAndBlockedTasks(t *testing.T) {
	t.Parallel()
	for _, status := range []domain.TaskStatus{domain.TaskDone, domain.TaskCanceled, domain.TaskBlocked} {
		task := readyTask()
		task.Status = status
		tasks := &blockingTasks{task: task}
		store := newFakeStore()
		worker := NewJobWorker(store, tasks, nil, nil, slog.New(slog.DiscardHandler))

		worker.blockTaskAfterFailure(context.Background(), domain.Run{
			ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: task.ID,
			Status: domain.RunFailed, FailureCode: "test_gate_failed",
		})

		if tasks.task.Status != status {
			t.Fatalf("%s task moved to %s", status, tasks.task.Status)
		}
		if len(store.events) != 0 {
			t.Fatalf("%s task recorded %d events", status, len(store.events))
		}
	}
}
