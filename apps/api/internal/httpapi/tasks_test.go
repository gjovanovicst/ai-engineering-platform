package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeTaskService struct {
	created    tasks.CreateInput
	transition string
	err        error
}

func (f *fakeTaskService) MarkMerged(context.Context, string, string, string) (domain.Task, error) {
	return domain.Task{ID: "t1", Status: domain.TaskDone}, nil
}

func (f *fakeTaskService) Create(_ context.Context, in tasks.CreateInput) (domain.Task, error) {
	if f.err != nil {
		return domain.Task{}, f.err
	}
	f.created = in
	return domain.Task{ID: "t1", Ref: "T-1", Title: in.Title, Status: domain.TaskDraft, Priority: domain.PriorityMed}, nil
}

func (f *fakeTaskService) Get(context.Context, string, string, string) (domain.Task, error) {
	return domain.Task{}, f.err
}

func (f *fakeTaskService) List(context.Context, string, string, string, string, int) ([]domain.Task, error) {
	return nil, f.err
}

func (f *fakeTaskService) Update(_ context.Context, in tasks.UpdateInput) (domain.Task, error) {
	if f.err != nil {
		return domain.Task{}, f.err
	}
	return domain.Task{ID: "t1", Title: in.Title, Priority: in.Priority, Status: domain.TaskDraft}, nil
}

func (f *fakeTaskService) Transition(_ context.Context, _ string, _, _, to, _ string) (domain.Task, error) {
	if f.err != nil {
		return domain.Task{}, f.err
	}
	f.transition = to
	return domain.Task{ID: "t1", Status: domain.TaskStatus(to)}, nil
}

func taskRouter(t *testing.T, svc TaskService) (http.Handler, *auth.Issuer) {
	t.Helper()
	issuer := testIssuer(t)
	return NewRouter(Deps{
		Version: "test", StartedAt: time.Now(), Log: discardLogger(),
		Issuer: issuer, Projects: &fakeProjectService{}, Tasks: svc,
	}), issuer
}

func TestTasksRequireAuth(t *testing.T) {
	router, _ := taskRouter(t, &fakeTaskService{})
	rec := doJSON(router, http.MethodGet, "/api/v1/orgs/org-1/projects/p1/tasks", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTasksCreate(t *testing.T) {
	svc := &fakeTaskService{}
	router, issuer := taskRouter(t, svc)
	token := tokenFor(t, issuer, "org-1", "member")

	body := `{"title":"Add SSO","description":"saml","priority":"high","acceptance":[{"given":"a user","when":"they click","then":"they are redirected"}]}`
	rec := doJSON(router, http.MethodPost, "/api/v1/orgs/org-1/projects/p1/tasks", token, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.created.Title != "Add SSO" || len(svc.created.Acceptance) != 1 {
		t.Fatalf("created = %+v", svc.created)
	}
}

func TestTasksTransitionConflict(t *testing.T) {
	svc := &fakeTaskService{err: &domain.TransitionError{From: domain.TaskPlanning, To: domain.TaskInProgress, Detail: "the plan has not been approved"}}
	router, issuer := taskRouter(t, svc)
	token := tokenFor(t, issuer, "org-1", "member")

	rec := doJSON(router, http.MethodPost, "/api/v1/orgs/org-1/projects/p1/tasks/T-1/transitions", token, `{"to":"in_progress"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_transition") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
