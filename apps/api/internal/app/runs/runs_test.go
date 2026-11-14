package runs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	runs     map[string]domain.Run
	events   []Event
	expired  []ExpiredApproval
	review   []byte
	approval *Approval
	decided  int
}

func newFakeStore() *fakeStore { return &fakeStore{runs: map[string]domain.Run{}} }

func (f *fakeStore) Create(_ context.Context, r domain.Run) (domain.Run, error) {
	r.ID = "run-1"
	r.CreatedAt = time.Now()
	f.runs[r.ID] = r
	return r, nil
}
func (f *fakeStore) Get(_ context.Context, _, id string) (domain.Run, error) {
	r, ok := f.runs[id]
	if !ok {
		return domain.Run{}, domain.ErrNotFound
	}
	return r, nil
}
func (f *fakeStore) UpdateStatus(_ context.Context, r domain.Run) (domain.Run, error) {
	f.runs[r.ID] = r
	return r, nil
}
func (f *fakeStore) AppendEvent(_ context.Context, _, _ string, typ, _ string, p map[string]any) error {
	f.events = append(f.events, Event{Type: typ, Payload: p})
	return nil
}
func (f *fakeStore) Events(context.Context, string, string) ([]Event, error) { return f.events, nil }

func (f *fakeStore) EventsAfter(context.Context, string, string, int64) ([]Event, error) {
	return f.events, nil
}
func (f *fakeStore) SetPlan(context.Context, string, string, []byte, string) error { return nil }
func (f *fakeStore) SetDiff(context.Context, string, string, []byte, string) error { return nil }
func (f *fakeStore) AddApprovalVote(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *fakeStore) CountApprovalVotes(context.Context, string, string) (int, error) { return 1, nil }

func (f *fakeStore) SetTraceID(context.Context, string, string, string) error { return nil }

func (f *fakeStore) SetContextHash(context.Context, string, string, string) error { return nil }

func (f *fakeStore) SetAcceptanceProof(context.Context, string, string, []byte) error { return nil }
func (f *fakeStore) SetReview(context.Context, string, string, []byte) error          { return nil }
func (f *fakeStore) LatestRun(context.Context, string, string, string) (domain.Run, error) {
	return domain.Run{}, domain.ErrNotFound
}
func (f *fakeStore) LatestForProject(context.Context, string, string) (domain.Run, error) {
	return domain.Run{}, domain.ErrNotFound
}

func (f *fakeStore) ListForTask(context.Context, string, string, int) ([]domain.Run, error) {
	return nil, nil
}
func (f *fakeStore) CreateApproval(context.Context, string, string, string, string, time.Time) (Approval, error) {
	return Approval{ID: "a1", Status: "pending"}, nil
}
func (f *fakeStore) GetApproval(context.Context, string, string) (Approval, error) {
	if f.approval != nil {
		return *f.approval, nil
	}
	return Approval{ID: "a1", Status: "pending", RunID: "run-1", OrgID: "o"}, nil
}
func (f *fakeStore) DecideApproval(context.Context, string, string, string, string, string, time.Time) (Approval, error) {
	f.decided++
	return Approval{ID: "a1", Status: "approved"}, nil
}
func (f *fakeStore) LatestReviewForTask(context.Context, string, string) ([]byte, error) {
	return f.review, nil
}
func (f *fakeStore) PendingApprovals(context.Context, string, int) ([]Approval, error) {
	return nil, nil
}

func (f *fakeStore) CountActive(context.Context, string) (int, error) { return 0, nil }

func (f *fakeStore) ExpireApprovals(context.Context, string) ([]ExpiredApproval, error) {
	return f.expired, nil
}

type fakeTasks struct{ task domain.Task }

func (f fakeTasks) Get(context.Context, string, string, string) (domain.Task, error) {
	return f.task, nil
}

func (f fakeTasks) UpdateStatus(_ context.Context, t domain.Task) (domain.Task, error) {
	return t, nil
}

type fakeExecutor struct{ called chan string }

func (f *fakeExecutor) Execute(_ context.Context, _ domain.Run, prompt string, _ []string, _ string, _ []string, _ []string, _ []ContextSource, _, _, _ string, _ float64, _ []visualdiff.Rect, _ int, _ []string, _ []string, _ []string, _ []PluginCheck) (domain.Run, error) {
	f.called <- prompt
	return domain.Run{}, nil
}

func readyTask() domain.Task {
	return domain.Task{ID: "task-1", Ref: "T-1", Title: "Add SSO", Status: domain.TaskReady,
		Acceptance: []domain.AcceptanceCriterion{{Given: "g", When: "w", Then: "t"}}}
}

func TestStartRejectsTaskThatIsNotReady(t *testing.T) {
	task := readyTask()
	task.Status = domain.TaskDraft
	exec := &fakeExecutor{called: make(chan string, 1)}
	svc, _ := New(newFakeStore(), fakeTasks{task: task}, exec, nil, nil, slog.New(slog.DiscardHandler))

	_, err := svc.Start(context.Background(), StartInput{OrgID: "o", ProjectID: "p", TaskID: "t", AgentType: "planning"})
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestStartRunsExecutorWithPrompt(t *testing.T) {
	exec := &fakeExecutor{called: make(chan string, 1)}
	svc, _ := New(newFakeStore(), fakeTasks{task: readyTask()}, exec, nil, nil, slog.New(slog.DiscardHandler))

	run, err := svc.Start(context.Background(), StartInput{OrgID: "o", ProjectID: "p", TaskID: "t", AgentType: "planning"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.RunQueued {
		t.Fatalf("status = %s, want queued", run.Status)
	}

	select {
	case prompt := <-exec.called:
		if !strings.Contains(prompt, "Add SSO") || !strings.Contains(prompt, "Given g") {
			t.Fatalf("prompt = %q", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("executor was not called")
	}
}

func TestCancelTerminalRun(t *testing.T) {
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", Status: domain.RunSucceeded}
	exec := &fakeExecutor{called: make(chan string, 1)}
	svc, _ := New(store, fakeTasks{task: readyTask()}, exec, nil, nil, slog.New(slog.DiscardHandler))

	if _, err := svc.Cancel(context.Background(), "o", "run-1"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestSweepExpiredRecordsEvents(t *testing.T) {
	store := newFakeStore()
	store.expired = []ExpiredApproval{{ID: "a1", RunID: "run-1", Type: "plan"}}
	exec := &fakeExecutor{}
	svc, err := New(store, fakeTasks{task: readyTask()}, exec, nil, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SweepExpired(context.Background(), "o"); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 1 || store.events[0].Type != "approval.expired" {
		t.Fatalf("events = %+v", store.events)
	}
	if store.events[0].Payload["approval_id"] != "a1" {
		t.Fatalf("payload = %+v", store.events[0].Payload)
	}
}

func TestDecideBlocksOnReviewBlocker(t *testing.T) {
	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "t1", Status: domain.RunSucceeded}
	store.approval = &Approval{ID: "a1", Type: "pr", Status: "pending", RunID: "run-1", OrgID: "o", PullRequestID: "pr1", Number: 7}
	store.review = []byte(`{"findings":[{"severity":"blocker","file":"main.go","line":12,"message":"sql injection"}]}`)
	svc, err := New(store, fakeTasks{task: readyTask()}, &fakeExecutor{}, nil, nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Decide(context.Background(), "o", "a1", "approved", "", "u1")
	if err == nil || !strings.Contains(err.Error(), "review blocker") {
		t.Fatalf("err = %v", err)
	}
	if store.decided != 0 {
		t.Fatal("approval must stay pending when the gate blocks")
	}
}

func TestContextManifestIsDeterministic(t *testing.T) {
	run := domain.Run{TaskID: "t1", AgentType: "development", Model: "m"}
	a := buildContextManifest(run, []string{"b.go", "a.go"}, []string{"r2@1", "r1@3"}, "ds@1", nil)
	b := buildContextManifest(run, []string{"a.go", "b.go"}, []string{"r1@3", "r2@1"}, "ds@1", nil)
	if a.hash != b.hash {
		t.Fatalf("hash depends on input order: %s vs %s", a.hash, b.hash)
	}
	c := buildContextManifest(run, []string{"a.go"}, []string{"r1@3", "r2@1"}, "ds@1", nil)
	if c.hash == a.hash {
		t.Fatal("different files must change the hash")
	}
}

func TestDesignPromptIncludesTokens(t *testing.T) {
	raw := []byte(`{"schema_version":"1","tokens":[{"name":"--brand","value":"#4f5eea","category":"color"}],"accessibility":["WCAG AA"]}`)
	out := DesignPrompt("Acme UI", 2, "abc123", raw)
	for _, want := range []string{"Acme UI v2", "--brand = #4f5eea", "WCAG AA", "approved components"} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt missing %q:\n%s", want, out)
		}
	}
}

func TestFrozenContextMismatchIsDetected(t *testing.T) {
	cases := []struct {
		frozen, current string
		want            bool
	}{
		{"", "abc", false},
		{"abc", "abc", false},
		{"abc", "def", true},
	}
	for _, tc := range cases {
		if got := contextMismatch(tc.frozen, tc.current); got != tc.want {
			t.Fatalf("frozen=%q current=%q got=%v want=%v", tc.frozen, tc.current, got, tc.want)
		}
	}
}

func TestSourceDelta(t *testing.T) {
	frozen := []ContextSource{{Kind: "task", ID: "t1", Version: "1", SHA256: "a"}}
	current := []ContextSource{{Kind: "task", ID: "t1", Version: "2", SHA256: "b"}, {Kind: "rule", ID: "r1"}}
	changed := sourceDelta(frozen, current)
	if len(changed) != 2 || changed[0] != "rule" || changed[1] != "task" {
		t.Fatalf("changed = %v", changed)
	}
}

func TestHashWorkspaceIgnoresTarMetadata(t *testing.T) {
	makeTar := func(order []string, mtime time.Time) []byte {
		var buf bytes.Buffer
		w := tar.NewWriter(&buf)
		for _, name := range order {
			body := []byte("content-" + name)
			_ = w.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), ModTime: mtime, Typeflag: tar.TypeReg})
			_, _ = w.Write(body)
		}
		_ = w.Close()
		return buf.Bytes()
	}
	a := makeTar([]string{"a.txt", "b.txt"}, time.Unix(1, 0))
	b := makeTar([]string{"b.txt", "a.txt"}, time.Unix(999, 0))
	if hashWorkspace(a) != hashWorkspace(b) {
		t.Fatal("workspace hash must ignore tar order/mtime")
	}
}

func TestContextHashIgnoresAgentType(t *testing.T) {
	run := domain.Run{TaskID: "t1", AgentType: "planning"}
	a := buildContextManifest(run, []string{"a.go"}, nil, "", nil)
	run.AgentType = "development"
	b := buildContextManifest(run, []string{"a.go"}, nil, "", nil)
	if a.hash != b.hash {
		t.Fatal("agent type must not change the context hash")
	}
}
