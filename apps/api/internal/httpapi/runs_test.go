package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeRunService struct {
	after     int64
	delivered bool
}

func (f *fakeRunService) LatestForProject(context.Context, string, string) (domain.Run, error) {
	return domain.Run{}, domain.ErrNotFound
}

func (f *fakeRunService) Start(context.Context, runs.StartInput) (domain.Run, error) {
	return domain.Run{ID: "run-1", Status: domain.RunQueued}, nil
}
func (f *fakeRunService) Retry(context.Context, runs.RetryInput) (domain.Run, error) {
	return domain.Run{ID: "run-2", Status: domain.RunQueued, AgentType: "development"}, nil
}
func (f *fakeRunService) FixReview(context.Context, runs.FixReviewInput) (domain.Run, error) {
	return domain.Run{ID: "run-3", Status: domain.RunQueued, AgentType: "development"}, nil
}
func (f *fakeRunService) FixCI(context.Context, string, string, string, string) (domain.Run, error) {
	return domain.Run{ID: "run-4", Status: domain.RunQueued, AgentType: "development"}, nil
}
func (f *fakeRunService) Get(context.Context, string, string) (domain.Run, error) {
	return domain.Run{ID: "run-1", Status: domain.RunSucceeded}, nil
}
func (f *fakeRunService) Events(context.Context, string, string) ([]runs.Event, error) {
	return nil, nil
}
func (f *fakeRunService) EventsAfter(_ context.Context, _, _ string, after int64) ([]runs.Event, error) {
	f.after = after
	if f.delivered {
		return nil, nil
	}
	f.delivered = true
	return []runs.Event{
		{Seq: 1, Type: "run.started", Actor: "system", Payload: map[string]any{"status": "running"}},
		{Seq: 2, Type: "assistant.text", Actor: "agent", Payload: map[string]any{"text": "hello"}},
	}, nil
}
func (f *fakeRunService) Cancel(context.Context, string, string) (domain.Run, error) {
	return domain.Run{ID: "run-1", Status: domain.RunCanceled}, nil
}
func (f *fakeRunService) PendingApprovals(context.Context, string, int) ([]runs.Approval, error) {
	return []runs.Approval{{ID: "a1", RunID: "run-1", Type: "plan", Status: "pending"}}, nil
}
func (f *fakeRunService) ListForTask(context.Context, string, string, int) ([]domain.Run, error) {
	return nil, nil
}

func (f *fakeRunService) Decide(context.Context, string, string, string, string, string) (domain.Run, string, error) {
	return domain.Run{ID: "run-1", Status: domain.RunSucceeded}, "dev-1", nil
}

func TestRunEventsSSE(t *testing.T) {
	issuer := testIssuer(t)
	svc := &fakeRunService{}
	router := NewRouter(Deps{Version: "test", StartedAt: time.Now(), Log: discardLogger(), Issuer: issuer, Runs: svc})
	server := httptest.NewServer(router)
	defer server.Close()

	token := tokenFor(t, issuer, "org-1", "viewer")
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/orgs/org-1/runs/run-1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)
	for _, want := range []string{"retry: 3000", "id: 1", "event: run.started", "id: 2", "event: assistant.text", `"text":"hello"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("stream missing %q:\n%s", want, out)
		}
	}
}

func TestRunEventsJSONByDefault(t *testing.T) {
	issuer := testIssuer(t)
	svc := &fakeRunService{}
	router := NewRouter(Deps{Version: "test", StartedAt: time.Now(), Log: discardLogger(), Issuer: issuer, Runs: svc})
	server := httptest.NewServer(router)
	defer server.Close()

	token := tokenFor(t, issuer, "org-1", "viewer")
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/orgs/org-1/runs/run-1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestRunJSONIncludesPlan(t *testing.T) {
	run := domain.Run{
		ID: "run-1", Status: domain.RunAwaitingApproval,
		Plan: []byte(`{"steps":["one"],"files":["a.ts"]}`),
	}
	out := runJSON(run)
	plan, ok := out["plan"].(json.RawMessage)
	if !ok {
		t.Fatalf("plan = %#v, want json.RawMessage", out["plan"])
	}
	if !strings.Contains(string(plan), `"steps"`) {
		t.Fatalf("plan = %s", plan)
	}
}

func (f *fakeRunService) Troubleshoot(context.Context, runs.TroubleshootInput) (domain.Run, error) {
	return domain.Run{}, nil
}
