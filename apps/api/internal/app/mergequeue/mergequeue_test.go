package mergequeue

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	items   []Item
	status  []string
	details []string
}

func (f *fakeStore) Enqueue(_ context.Context, item Item) (int64, error) { return 1, nil }
func (f *fakeStore) ClaimNext(_ context.Context, _, _ string) (Item, bool, error) {
	if len(f.items) == 0 {
		return Item{}, false, nil
	}
	item := f.items[0]
	f.items = f.items[1:]
	return item, true, nil
}
func (f *fakeStore) Complete(_ context.Context, _ string, _ int64, status, detail string) error {
	f.status = append(f.status, status)
	f.details = append(f.details, detail)
	return nil
}
func (f *fakeStore) Position(context.Context, string, int64) (int, error) { return 1, nil }
func (f *fakeStore) List(context.Context, string, string) ([]Item, error) { return nil, nil }

type fakeMerger struct {
	numbers []int
	failOn  int
}

func (f *fakeMerger) Merge(_ context.Context, _, _ string, number int) error {
	f.numbers = append(f.numbers, number)
	if number == f.failOn {
		return errors.New("conflict")
	}
	return nil
}

func TestProcessAllIsOrderedAndContinuesAfterFailure(t *testing.T) {
	store := &fakeStore{items: []Item{{Number: 1}, {Number: 2}, {Number: 3}}}
	merger := &fakeMerger{failOn: 2}
	svc := New(store, merger, nil)
	results, err := svc.ProcessAll(context.Background(), "o", "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(merger.numbers) != 3 || merger.numbers[0] != 1 || merger.numbers[2] != 3 {
		t.Fatalf("order = %v", merger.numbers)
	}
	if len(results) != 3 || results[1].Err == nil {
		t.Fatalf("results = %+v", results)
	}
	if store.status[1] != "failed" || store.status[0] != "merged" {
		t.Fatalf("status = %v", store.status)
	}
}

type fakeRebaser struct {
	fakeMerger
	rebaseErr error
}

func (f *fakeRebaser) Rebase(context.Context, string, string, int) error { return f.rebaseErr }

type fakeGate struct {
	blocked bool
}

func (f fakeGate) Blocked(context.Context, string, string) (bool, string, error) {
	return f.blocked, "test check failed", nil
}

func TestGateFailureSkipsMerge(t *testing.T) {
	store := &fakeStore{items: []Item{{Number: 9, PullRequestID: "pr"}}}
	merger := &fakeMerger{}
	results, err := New(store, merger, nil).WithGate(fakeGate{blocked: true}).ProcessAll(context.Background(), "o", "p")
	if err != nil || len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if len(merger.numbers) != 0 || store.status[0] != "failed" {
		t.Fatalf("numbers=%v status=%v", merger.numbers, store.status)
	}
}

func TestRebaseFailureSkipsMerge(t *testing.T) {
	store := &fakeStore{items: []Item{{Number: 4}}}
	merger := &fakeRebaser{rebaseErr: errors.New("conflict")}
	results, err := New(store, merger, nil).ProcessAll(context.Background(), "o", "p")
	if err != nil || len(results) != 1 || results[0].Err == nil {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if len(merger.numbers) != 0 || store.status[0] != "failed" {
		t.Fatalf("numbers=%v status=%v", merger.numbers, store.status)
	}
}

func TestTenQueuedPRsAreProcessedOnceInOrder(t *testing.T) {
	items := make([]Item, 0, 10)
	for i := 1; i <= 10; i++ {
		items = append(items, Item{Number: i})
	}
	store := &fakeStore{items: items}
	merger := &fakeMerger{}
	svc := New(store, merger, nil)
	results, err := svc.ProcessAll(context.Background(), "org", "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 || len(merger.numbers) != 10 {
		t.Fatalf("results=%d merges=%d", len(results), len(merger.numbers))
	}
	seen := map[int]bool{}
	for i, number := range merger.numbers {
		if number != i+1 || seen[number] {
			t.Fatalf("order/duplicate: %v", merger.numbers)
		}
		seen[number] = true
	}
}
