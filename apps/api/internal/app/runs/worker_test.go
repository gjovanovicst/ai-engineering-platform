package runs

import (
	"context"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/queue"
)

type fakeBackend struct {
	runs     int
	lastSpec agent.Spec
}

func (f *fakeBackend) Name() string { return "fake" }
func (f *fakeBackend) Run(_ context.Context, spec agent.Spec, emit func(agent.Event) error) (agent.Result, error) {
	f.runs++
	f.lastSpec = spec
	return agent.Result{}, emit(agent.Event{Type: agent.EventDone})
}

func TestJobWorkerExecutesQueuedRun(t *testing.T) {
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "t", Status: domain.RunQueued}
	backend := &fakeBackend{}
	runner := NewRunner(backend, store, nil, nil, "", slog.New(slog.DiscardHandler))
	worker := NewJobWorker(store, fakeTasks{task: readyTask()}, nil, runner, slog.New(slog.DiscardHandler))

	if err := worker.Work(context.Background(), &river.Job[queue.RunArgs]{
		Args: queue.RunArgs{RunID: "run-1", OrgID: "o"},
	}); err != nil {
		t.Fatal(err)
	}
	if backend.runs != 1 {
		t.Fatalf("backend runs = %d, want 1", backend.runs)
	}
	if got := store.runs["run-1"].Status; got != domain.RunSucceeded {
		t.Fatalf("status = %s, want succeeded", got)
	}
}

func TestJobWorkerSkipsTerminalRun(t *testing.T) {
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", OrgID: "o", Status: domain.RunSucceeded}
	backend := &fakeBackend{}
	runner := NewRunner(backend, store, nil, nil, "", slog.New(slog.DiscardHandler))
	worker := NewJobWorker(store, fakeTasks{task: readyTask()}, nil, runner, slog.New(slog.DiscardHandler))

	if err := worker.Work(context.Background(), &river.Job[queue.RunArgs]{
		Args: queue.RunArgs{RunID: "run-1", OrgID: "o"},
	}); err != nil {
		t.Fatal(err)
	}
	if backend.runs != 0 {
		t.Fatalf("backend runs = %d, want 0", backend.runs)
	}
}

type fakeWorkspace struct {
	archive []byte
	err     error
	calls   int
}

func (f *fakeWorkspace) Clone(context.Context, string, string) ([]byte, error) {
	f.calls++
	return f.archive, f.err
}

func TestRunnerUploadsClonedWorkspace(t *testing.T) {
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{
		ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "t",
		Status: domain.RunQueued, AgentType: string(domain.AgentDevelopment),
	}
	backend := &fakeBackend{}
	workspace := &fakeWorkspace{archive: []byte("tar-bytes")}
	runner := NewRunner(backend, store, nil, nil, "", slog.New(slog.DiscardHandler)).
		WithWorkspaceProvider(workspace)
	worker := NewJobWorker(store, fakeTasks{task: readyTask()}, nil, runner, slog.New(slog.DiscardHandler))

	// Development without a diff fails the run, but the backend must still have
	// received the cloned archive.
	_ = worker.Work(context.Background(), &river.Job[queue.RunArgs]{
		Args: queue.RunArgs{RunID: "run-1", OrgID: "o"},
	})
	if workspace.calls != 1 {
		t.Fatalf("workspace calls = %d, want 1", workspace.calls)
	}
	if string(backend.lastSpec.Workspace) != "tar-bytes" {
		t.Fatalf("workspace = %q, want the cloned archive", backend.lastSpec.Workspace)
	}
}
