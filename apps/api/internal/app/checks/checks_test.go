package checks

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	checks   []Check
	failed   bool
	required []string
}

func (f *fakeStore) Upsert(_ context.Context, _, _, _, _ string, c Check) (Check, error) {
	f.checks = append(f.checks, c)
	return c, nil
}
func (f *fakeStore) ListForPullRequest(context.Context, string, string) ([]Check, error) {
	return f.checks, nil
}
func (f *fakeStore) ListForRun(context.Context, string, string) ([]Check, error) {
	return f.checks, nil
}
func (f *fakeStore) Failed(context.Context, string, string) (bool, error) { return f.failed, nil }
func (f *fakeStore) RequiredForPullRequest(context.Context, string, string) ([]string, error) {
	return f.required, nil
}
func (f *fakeStore) InsertForRun(_ context.Context, _, _ string, c Check) (Check, error) {
	f.checks = append(f.checks, c)
	return c, nil
}

func TestRecordValidates(t *testing.T) {
	svc := New(&fakeStore{}, nil)
	if _, err := svc.Record(context.Background(), "o", "r", "p", "", Check{Name: ""}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty name err = %v", err)
	}
	if _, err := svc.Record(context.Background(), "o", "r", "p", "", Check{Name: "x", Status: "weird"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad status err = %v", err)
	}
	got, err := svc.Record(context.Background(), "o", "r", "p", "", Check{Name: "test", Status: "passed"})
	if err != nil || got.Status != "passed" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestBlockedByFailedAndRequiredChecks(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, nil)
	blocked, _, err := svc.Blocked(context.Background(), "o", "p")
	if err != nil || blocked {
		t.Fatalf("empty gate blocked=%v err=%v", blocked, err)
	}

	store.checks = []Check{{Name: "ci", Status: "failed"}}
	blocked, reason, _ := svc.Blocked(context.Background(), "o", "p")
	if !blocked || reason != "check ci failed" {
		t.Fatalf("failed gate blocked=%v reason=%q", blocked, reason)
	}

	store.checks = []Check{{Name: "ci", Status: "passed"}}
	store.required = []string{"ci"}
	blocked, _, _ = svc.Blocked(context.Background(), "o", "p")
	if blocked {
		t.Fatal("passed required check must not block")
	}

	store.required = []string{"deploy"}
	blocked, reason, _ = svc.Blocked(context.Background(), "o", "p")
	if !blocked || reason == "" {
		t.Fatalf("missing required check blocked=%v reason=%q", blocked, reason)
	}

	store.checks = []Check{{Name: "deploy", Status: "pending"}}
	blocked, reason, _ = svc.Blocked(context.Background(), "o", "p")
	if !blocked || reason != "required check deploy is pending" {
		t.Fatalf("pending required blocked=%v reason=%q", blocked, reason)
	}
}
