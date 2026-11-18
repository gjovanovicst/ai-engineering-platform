package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projects"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeProjectService struct {
	items   []domain.Project
	created projects.CreateInput
	err     error
}

func (f *fakeProjectService) Create(_ context.Context, in projects.CreateInput) (domain.Project, error) {
	if f.err != nil {
		return domain.Project{}, f.err
	}
	f.created = in
	return domain.Project{ID: "p1", OrgID: in.OrgID, Name: in.Name, Slug: in.Slug, Status: domain.ProjectActive}, nil
}

func (f *fakeProjectService) Get(context.Context, string, string) (domain.Project, error) {
	if f.err != nil {
		return domain.Project{}, f.err
	}
	return domain.Project{ID: "p1", Name: "CRM", Slug: "crm", Status: domain.ProjectActive}, nil
}

func (f *fakeProjectService) UpdateRequiredChecks(context.Context, string, string, []string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetDesignStandard(_ context.Context, _, _, standardID string) (domain.Project, error) {
	return domain.Project{DesignStandardID: standardID}, f.err
}

func (f *fakeProjectService) DesignStandard(context.Context, string, string) (projects.ProjectStandard, error) {
	return projects.ProjectStandard{}, f.err
}

func (f *fakeProjectService) ListComponents(context.Context, string, string) ([]projects.ProjectComponent, error) {
	return nil, f.err
}

func (f *fakeProjectService) Delete(context.Context, string, string, string, string) error {
	return f.err
}

func (f *fakeProjectService) SetPluginChecks(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetRequiredApprovals(context.Context, string, string, int) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetAllowedTools(context.Context, string, string, []string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetAllowedModels(context.Context, string, string, []string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetChangeWindow(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetPolicy(context.Context, string, string, int, []string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetVisualPolicy(context.Context, string, string, float64, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetMCPConfig(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetVisualCommand(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetE2ECommand(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) SetTestCommand(context.Context, string, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeProjectService) ReplaceComponents(context.Context, string, string, string, []projects.ComponentSelection) (int, error) {
	return 0, f.err
}

func (f *fakeProjectService) List(context.Context, string, int) ([]domain.Project, error) {
	return f.items, f.err
}

func projectRouter(t *testing.T, svc ProjectService) (http.Handler, *auth.Issuer) {
	t.Helper()
	issuer := testIssuer(t)
	return NewRouter(Deps{
		Version: "test", StartedAt: time.Now(), Log: discardLogger(),
		Issuer: issuer, Projects: svc,
	}), issuer
}

func tokenFor(t *testing.T, issuer *auth.Issuer, org, role string) string {
	t.Helper()
	raw, _, err := issuer.Issue("user-1", auth.Claims{OrgID: org, Role: role, SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func doJSON(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestProjectsRequireAuth(t *testing.T) {
	router, _ := projectRouter(t, &fakeProjectService{})
	rec := doJSON(router, http.MethodGet, "/api/v1/orgs/acme/projects", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProjectsOrgMustMatchToken(t *testing.T) {
	router, issuer := projectRouter(t, &fakeProjectService{})
	token := tokenFor(t, issuer, "org-1", "owner")
	rec := doJSON(router, http.MethodGet, "/api/v1/orgs/org-2/projects", token, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestProjectsCreateRequiresMember(t *testing.T) {
	router, issuer := projectRouter(t, &fakeProjectService{})
	token := tokenFor(t, issuer, "org-1", "viewer")
	rec := doJSON(router, http.MethodPost, "/api/v1/orgs/org-1/projects", token, `{"name":"CRM"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestProjectsCreate(t *testing.T) {
	svc := &fakeProjectService{}
	router, issuer := projectRouter(t, svc)
	token := tokenFor(t, issuer, "org-1", "member")

	rec := doJSON(router, http.MethodPost, "/api/v1/orgs/org-1/projects", token,
		`{"name":"CRM Platform","description":"customer relations"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if svc.created.OrgID != "org-1" || svc.created.Name != "CRM Platform" {
		t.Fatalf("created = %+v", svc.created)
	}
	if !strings.Contains(rec.Body.String(), `"slug":""`) && !strings.Contains(rec.Body.String(), `"slug"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestProjectsList(t *testing.T) {
	svc := &fakeProjectService{items: []domain.Project{
		{ID: "p1", Name: "CRM", Slug: "crm", Status: domain.ProjectActive},
		{ID: "p2", Name: "Portal", Slug: "portal", Status: domain.ProjectActive},
	}}
	router, issuer := projectRouter(t, svc)
	token := tokenFor(t, issuer, "org-1", "viewer")

	rec := doJSON(router, http.MethodGet, "/api/v1/orgs/org-1/projects?limit=10", token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"slug":"portal"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
