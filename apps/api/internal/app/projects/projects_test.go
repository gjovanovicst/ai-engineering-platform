package projects

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	created domain.Project
	err     error
	limit   int
}

func (f *fakeStore) Create(_ context.Context, p domain.Project) (domain.Project, error) {
	if f.err != nil {
		return domain.Project{}, f.err
	}
	f.created = p
	p.ID = "p1"
	return p, nil
}

func (f *fakeStore) Get(context.Context, string, string) (domain.Project, error) {
	return domain.Project{}, f.err
}

func (f *fakeStore) UpdateRequiredChecks(_ context.Context, _, _ string, checks []string) (domain.Project, error) {
	return domain.Project{RequiredChecks: checks}, nil
}

func (f *fakeStore) SetDesignStandard(_ context.Context, _, _, standardID string) (domain.Project, error) {
	return domain.Project{DesignStandardID: standardID}, nil
}

func (f *fakeStore) DesignStandard(context.Context, string, string) (ProjectStandard, error) {
	return ProjectStandard{}, domain.ErrNotFound
}

func (f *fakeStore) ListComponents(context.Context, string, string) ([]ProjectComponent, error) {
	return nil, nil
}

func (f *fakeStore) Delete(context.Context, string, string) error { return nil }

func (f *fakeStore) Audit(context.Context, string, string, string, string, []byte) error { return nil }

func (f *fakeStore) SetPluginChecks(_ context.Context, _, _, checks string) (domain.Project, error) {
	return domain.Project{PluginChecks: checks}, nil
}

func (f *fakeStore) SetRequiredApprovals(_ context.Context, _, _ string, n int) (domain.Project, error) {
	return domain.Project{PolicyRequiredApprovals: n}, nil
}

func (f *fakeStore) SetAllowedTools(_ context.Context, _, _ string, tools []string) (domain.Project, error) {
	return domain.Project{PolicyAllowedTools: tools}, nil
}

func (f *fakeStore) SetAllowedModels(_ context.Context, _, _ string, models []string) (domain.Project, error) {
	return domain.Project{PolicyAllowedModels: models}, nil
}

func (f *fakeStore) SetChangeWindow(_ context.Context, _, _, window string) (domain.Project, error) {
	return domain.Project{PolicyChangeWindow: window}, nil
}

func (f *fakeStore) SetPolicy(_ context.Context, _, _ string, max int, paths []string) (domain.Project, error) {
	return domain.Project{PolicyMaxDiffBytes: max, PolicyForbiddenPaths: paths}, nil
}

func (f *fakeStore) SetVisualPolicy(_ context.Context, _, _ string, threshold float64, masks string) (domain.Project, error) {
	return domain.Project{VisualThreshold: threshold, VisualMasks: masks}, nil
}

func (f *fakeStore) SetMCPConfig(_ context.Context, _, _, config string) (domain.Project, error) {
	return domain.Project{MCPServers: config}, nil
}

func (f *fakeStore) SetVisualCommand(_ context.Context, _, _, command string) (domain.Project, error) {
	return domain.Project{VisualCommand: command}, nil
}

func (f *fakeStore) SetE2ECommand(_ context.Context, _, _, command string) (domain.Project, error) {
	return domain.Project{E2ECommand: command}, nil
}

func (f *fakeStore) SetTestCommand(_ context.Context, _, _, command string) (domain.Project, error) {
	return domain.Project{TestCommand: command}, nil
}

func (f *fakeStore) ReplaceComponents(context.Context, string, string, string, []ComponentSelection) (int, error) {
	return 0, nil
}

func (f *fakeStore) List(_ context.Context, _ string, limit int) ([]domain.Project, error) {
	f.limit = limit
	return nil, f.err
}

func TestCreateDerivesSlug(t *testing.T) {
	store := &fakeStore{}
	svc, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.Create(context.Background(), CreateInput{OrgID: "org-1", Name: "CRM Platform"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "crm-platform" {
		t.Fatalf("slug = %q, want crm-platform", p.Slug)
	}
}

func TestCreateRejectsInvalidProject(t *testing.T) {
	svc, err := New(&fakeStore{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(context.Background(), CreateInput{OrgID: "org-1", Name: ""})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSetTestCommandTrimsAndValidates(t *testing.T) {
	t.Parallel()
	svc, err := New(&fakeStore{})
	if err != nil {
		t.Fatal(err)
	}
	project, err := svc.SetTestCommand(context.Background(), "org-1", "proj-1", "  make check  ")
	if err != nil {
		t.Fatal(err)
	}
	if project.TestCommand != "make check" {
		t.Fatalf("test command = %q, want %q", project.TestCommand, "make check")
	}
	long := strings.Repeat("x", 501)
	if _, err := svc.SetTestCommand(context.Background(), "org-1", "proj-1", long); err == nil {
		t.Fatal("expected a validation error for a command longer than 500 characters")
	}
}

func TestListClampsLimit(t *testing.T) {
	store := &fakeStore{}
	svc, err := New(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(context.Background(), "org-1", 500); err != nil {
		t.Fatal(err)
	}
	if store.limit != 100 {
		t.Fatalf("limit = %d, want 100", store.limit)
	}
	if _, err := svc.List(context.Background(), "org-1", 0); err != nil {
		t.Fatal(err)
	}
	if store.limit != 50 {
		t.Fatalf("limit = %d, want 50", store.limit)
	}
}

func (f *fakeStore) Update(_ context.Context, _, _ string, _, _ *string) (domain.Project, error) {
	return domain.Project{}, f.err
}
