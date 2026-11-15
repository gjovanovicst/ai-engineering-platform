package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeRepos struct{}

func (f *fakeRepos) Create(context.Context, domain.Repo) (domain.Repo, error) {
	return domain.Repo{}, nil
}
func (f *fakeRepos) List(context.Context, string, string) ([]domain.Repo, error) { return nil, nil }
func (f *fakeRepos) Get(context.Context, string, string, string) (domain.Repo, error) {
	return domain.Repo{}, domain.ErrNotFound
}
func (f *fakeRepos) ForWebhookName(context.Context, string, string) (domain.Repo, error) {
	return domain.Repo{}, nil
}

func (f *fakeRepos) ForWebhook(context.Context, string) (domain.Repo, error) {
	return domain.Repo{ID: "repo-1", OrgID: "org-1", ProjectID: "proj-1", Provider: domain.ProviderGitHub, WebhookSecret: "s3cret"}, nil
}
func (f *fakeRepos) RecordPullRequest(context.Context, domain.PullRequest) (domain.PullRequest, error) {
	return domain.PullRequest{}, nil
}
func (f *fakeRepos) PullRequestForMerge(context.Context, string, string, int) (domain.PullRequest, error) {
	return domain.PullRequest{ID: "pr-1", TaskID: "task-1", Number: 7}, nil
}
func (f *fakeRepos) SetPullRequestState(context.Context, string, string, string) error { return nil }
func (f *fakeRepos) ApprovedForPR(context.Context, string, string) (bool, error)       { return true, nil }
func (f *fakeRepos) PullRequestsForProject(context.Context, string, string, int) ([]domain.PullRequest, error) {
	return nil, nil
}

type fakeMergeTasks struct {
	merged []string
}

func (f *fakeMergeTasks) Create(context.Context, tasks.CreateInput) (domain.Task, error) {
	return domain.Task{}, nil
}
func (f *fakeMergeTasks) Get(context.Context, string, string, string) (domain.Task, error) {
	return domain.Task{}, nil
}
func (f *fakeMergeTasks) List(context.Context, string, string, string, string, int) ([]domain.Task, error) {
	return nil, nil
}
func (f *fakeMergeTasks) Transition(context.Context, string, string, string, string, string) (domain.Task, error) {
	return domain.Task{}, nil
}

func (f *fakeMergeTasks) Update(context.Context, tasks.UpdateInput) (domain.Task, error) {
	return domain.Task{ID: "t1", Status: domain.TaskDraft}, nil
}

func (f *fakeMergeTasks) MarkMerged(_ context.Context, _, _, idOrRef string) (domain.Task, error) {
	f.merged = append(f.merged, idOrRef)
	return domain.Task{ID: idOrRef, Status: domain.TaskDone}, nil
}

func TestGitHubWebhookMergesTask(t *testing.T) {
	repos := &fakeRepos{}
	tasks := &fakeMergeTasks{}
	router := NewRouter(Deps{Version: "test", StartedAt: time.Now(), Log: discardLogger(), Repos: repos, Tasks: tasks})

	body := []byte(`{"action":"closed","number":7,"pull_request":{"merged":true},"repository":{"full_name":"o/r"}}`)
	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/repo-1", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", signature)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(tasks.merged) != 1 || tasks.merged[0] != "task-1" {
		t.Fatalf("merged = %v", tasks.merged)
	}

	// A wrong signature must be rejected and must not touch the task.
	bad := httptest.NewRequest(http.MethodPost, "/webhooks/github/repo-1", strings.NewReader(string(body)))
	bad.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, bad)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(tasks.merged) != 1 {
		t.Fatalf("merged after bad signature = %v", tasks.merged)
	}
}

func (f *fakeRepos) PullRequest(context.Context, string, string) (domain.PullRequest, error) {
	return domain.PullRequest{ID: "pr-1", TaskID: "task-1", Number: 7}, nil
}
