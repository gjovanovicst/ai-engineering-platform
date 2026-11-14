package runs

import (
	"context"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type proofBackend struct {
	lines string
	diff  string
}

func (f *proofBackend) Name() string { return "fake" }
func (f *proofBackend) Run(_ context.Context, _ agent.Spec, emit func(agent.Event) error) (agent.Result, error) {
	for _, line := range strings.Split(f.lines, "\n") {
		if err := emit(agent.Event{Type: agent.EventText, Text: line}); err != nil {
			return agent.Result{}, err
		}
	}
	return agent.Result{Diff: f.diff, TestExitCode: 0}, nil
}

type fakeTests struct {
	name, status, output string
}

func (f *fakeTests) RecordRunTest(_ context.Context, _, _, name, status, output string) error {
	f.name, f.status, f.output = name, status, output
	return nil
}

func acceptanceRun() domain.Run {
	return domain.Run{
		ID: "run-a", OrgID: "o", ProjectID: "p", TaskID: "t",
		Status: domain.RunQueued, AgentType: string(domain.AgentDevelopment),
		Model: "test", Budget: domain.Budget{TokensIn: 1000, TokensOut: 1000},
	}
}

func TestRunnerRequiresProofForEveryCriterion(t *testing.T) {
	store := newFakeStore()
	tests := &fakeTests{}
	diff := "+++ b/pagination_test.go\n+func TestPager(t *testing.T) {}\n+++ b/pagination.go\n+func Page() {}\n"
	backend := &proofBackend{
		lines: `AIP_PROOF: {"evidence":[{"criterion":"AC1","kind":"automated","test":"TestPager","files":["pagination_test.go"]}]}`,
		diff:  diff,
	}
	runner := NewRunner(backend, store, nil, nil, "go test ./...", nil).WithTests(tests)
	run, err := runner.Execute(context.Background(), acceptanceRun(), "prompt", nil, "", nil, []string{"AC1"}, nil, "", "", "", 0.01, nil, 0, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.RunSucceeded {
		t.Fatalf("status = %s (%s: %s)", run.Status, run.FailureCode, run.FailureDetail)
	}
	if tests.name != "acceptance" || tests.status != "passed" {
		t.Fatalf("acceptance = %s/%s", tests.name, tests.status)
	}
}

func TestRunnerFailsWhenProofIsMissing(t *testing.T) {
	store := newFakeStore()
	tests := &fakeTests{}
	backend := &proofBackend{
		lines: "I changed the code.",
		diff:  "+++ b/pagination.go\n+func Page() {}\n",
	}
	runner := NewRunner(backend, store, nil, nil, "go test ./...", nil).WithTests(tests)
	run, err := runner.Execute(context.Background(), acceptanceRun(), "prompt", nil, "", nil, []string{"AC1"}, nil, "", "", "", 0.01, nil, 0, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != domain.RunFailed || run.FailureCode != "acceptance_unproven" {
		t.Fatalf("status = %s code = %s", run.Status, run.FailureCode)
	}
	if tests.status != "failed" {
		t.Fatalf("acceptance status = %s", tests.status)
	}
}

func TestManualEvidenceListsManualCriteria(t *testing.T) {
	proof := []byte(`{"evidence":[{"criterion":"AC1","kind":"automated"},{"criterion":"AC2","kind":"manual","reason":"reviewed"}]}`)
	got := manualEvidence(proof)
	if len(got) != 1 || got[0] != "AC2" {
		t.Fatalf("manual = %v", got)
	}
}
