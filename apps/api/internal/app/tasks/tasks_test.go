package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	count  int
	create func(domain.Task) (domain.Task, error)
	task   domain.Task
}

func (f *fakeStore) Create(_ context.Context, t domain.Task) (domain.Task, error) {
	if f.create != nil {
		return f.create(t)
	}
	t.ID = "t1"
	return t, nil
}
func (f *fakeStore) Get(context.Context, string, string) (domain.Task, error) {
	return f.task, nil
}
func (f *fakeStore) GetByRef(context.Context, string, string, string) (domain.Task, error) {
	return f.task, nil
}
func (f *fakeStore) List(context.Context, string, string, string, string, int) ([]domain.Task, error) {
	return nil, nil
}
func (f *fakeStore) Count(context.Context, string, string) (int, error) { return f.count, nil }
func (f *fakeStore) NextTaskRef(context.Context, string, string) (int64, error) {
	f.count++
	return int64(f.count), nil
}
func (f *fakeStore) Update(_ context.Context, t domain.Task) (domain.Task, error) {
	return t, nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, t domain.Task) (domain.Task, error) {
	return t, nil
}

func TestCreateAssignsNextRef(t *testing.T) {
	store := &fakeStore{count: 41}
	svc, _ := New(store)
	task, err := svc.Create(context.Background(), CreateInput{
		OrgID: "org-1", ProjectID: "p1", Title: "Add SSO",
		Acceptance: []domain.AcceptanceCriterion{{Given: "g", When: "w", Then: "t"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.Ref != "T-42" {
		t.Fatalf("ref = %q, want T-42", task.Ref)
	}
}

func TestCreateRetriesOnRefConflict(t *testing.T) {
	attempts := 0
	store := &fakeStore{count: 0}
	store.create = func(t domain.Task) (domain.Task, error) {
		attempts++
		if attempts == 1 {
			return domain.Task{}, domain.ErrConflict
		}
		t.ID = "t1"
		return t, nil
	}
	svc, _ := New(store)
	task, err := svc.Create(context.Background(), CreateInput{OrgID: "org-1", ProjectID: "p1", Title: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if task.Ref != "T-2" || attempts != 2 {
		t.Fatalf("ref = %q attempts = %d", task.Ref, attempts)
	}
}

func TestTransitionRejectsMissingFacts(t *testing.T) {
	store := &fakeStore{task: domain.Task{ID: "t1", OrgID: "org-1", Status: domain.TaskPlanning}}
	svc, _ := New(store)
	_, err := svc.Transition(context.Background(), "org-1", "t1", "p1", "in_progress", "")
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
}

func TestMarkMergedFromInProgress(t *testing.T) {
	store := &fakeStore{task: domain.Task{ID: "t1", OrgID: "o", ProjectID: "p", Ref: "T-1", Status: domain.TaskInProgress, Title: "x"}}
	svc, _ := New(store)
	merged, err := svc.MarkMerged(context.Background(), "o", "p", "T-1")
	if err != nil {
		t.Fatal(err)
	}
	if merged.Status != domain.TaskDone {
		t.Fatalf("status = %s, want done", merged.Status)
	}
}

func TestDependenciesBlocked(t *testing.T) {
	store := &fakeStore{task: domain.Task{ID: "t1", Ref: "T-1", Status: domain.TaskDone}}
	store.task.DependsOn = []string{"t1"}
	svc, _ := New(store)
	blockedBy, err := svc.DependenciesBlocked(context.Background(), "o", "p", "t2")
	if err != nil || blockedBy != "" {
		t.Fatalf("done dep: %q %v", blockedBy, err)
	}
	store.task.Status = domain.TaskReady
	blockedBy, err = svc.DependenciesBlocked(context.Background(), "o", "p", "t2")
	if err != nil || blockedBy != "T-1" {
		t.Fatalf("pending dep: %q %v", blockedBy, err)
	}
}
