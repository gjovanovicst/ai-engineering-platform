package blueprints

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

func validSpec() json.RawMessage {
	return json.RawMessage(`{
		"schema_version":"1","name":"Novasoft","description":"std",
		"stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},
		"repository":{"layout":"monorepo"},
		"test_strategy":{"unit":"go test ./..."},
		"agents":{"planning":{}}
	}`)
}

func TestParseAcceptsValidSpec(t *testing.T) {
	spec, err := Parse(validSpec())
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "Novasoft" || spec.Stack.Backend.Language != "go" {
		t.Fatalf("spec = %+v", spec)
	}
}

func TestParseRejectsInvalidSpecs(t *testing.T) {
	cases := map[string]string{
		"bad schema version": `{"schema_version":"3","name":"abc","stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},"repository":{"layout":"monorepo"},"test_strategy":{},"agents":{}}`,
		"bad backend":        `{"schema_version":"1","name":"abc","stack":{"backend":{"language":"rust"},"frontend":{"framework":"next","language":"typescript"}},"repository":{"layout":"monorepo"},"test_strategy":{},"agents":{}}`,
		"bad layout":         `{"schema_version":"1","name":"abc","stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},"repository":{"layout":"split"},"test_strategy":{},"agents":{}}`,
		"missing tests":      `{"schema_version":"1","name":"abc","stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},"repository":{"layout":"monorepo"},"agents":{}}`,
		"short name":         `{"schema_version":"1","name":"ab","stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},"repository":{"layout":"monorepo"},"test_strategy":{},"agents":{}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(json.RawMessage(raw))
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(err.Error(), "blueprint:") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseAcceptsStarterTasks(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_version":"2","name":"Novasoft",
		"stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},
		"repository":{"layout":"monorepo"},"test_strategy":{"unit":"x"},"agents":{"p":{}},
		"starter_tasks":[{"title":"Set up CI","priority":"p1","acceptance":[{"given":"a PR","when":"opened","then":"CI runs"}]}]
	}`)
	spec, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.StarterTasks) != 1 || spec.StarterTasks[0].Title != "Set up CI" {
		t.Fatalf("starter tasks = %+v", spec.StarterTasks)
	}
}

func TestParseRejectsBadStarterTask(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_version":"2","name":"Novasoft",
		"stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},
		"repository":{"layout":"monorepo"},"test_strategy":{"unit":"x"},"agents":{"p":{}},
		"starter_tasks":[{"title":"x","priority":"p9"}]
	}`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("invalid starter task accepted")
	}
}

func TestInstantiateCreatesStarterTasks(t *testing.T) {
	store := &fakeStore{blueprint: Blueprint{
		ID: "bp-1", OrgID: "o", Status: "published",
		Spec: json.RawMessage(`{
			"schema_version":"2","name":"Novasoft",
			"stack":{"backend":{"language":"go"},"frontend":{"framework":"next","language":"typescript"}},
			"repository":{"layout":"monorepo"},"test_strategy":{"unit":"x"},"agents":{"p":{}},
			"starter_tasks":[{"title":"Set up CI","priority":"p0"},{"title":"Add lint","priority":"p2"}]
		}`),
	}}
	var created []string
	var priorities []domain.Priority
	svc, err := New(store,
		func(context.Context, string, string, string, string) (domain.Project, error) {
			return domain.Project{ID: "p-1", Name: "Novasoft"}, nil
		},
		func(_ context.Context, _, _, title, _ string, priority domain.Priority, _ []domain.AcceptanceCriterion) (domain.Task, error) {
			created = append(created, title)
			priorities = append(priorities, priority)
			return domain.Task{ID: title}, nil
		}, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, count, err := svc.Instantiate(context.Background(), "o", "bp-1")
	if err != nil {
		t.Fatal(err)
	}
	if project.ID != "p-1" || count != 2 || len(created) != 2 {
		t.Fatalf("project=%+v count=%d created=%v", project, count, created)
	}
	if priorities[0] != domain.PriorityHigh || priorities[1] != domain.PriorityLow {
		t.Fatalf("priorities = %v", priorities)
	}
}

func TestParseRejectsNonJSON(t *testing.T) {
	if _, err := Parse(json.RawMessage("{")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

type fakeStore struct{ blueprint Blueprint }

func (f *fakeStore) Create(context.Context, Blueprint) (Blueprint, error) { return f.blueprint, nil }
func (f *fakeStore) List(context.Context, string, int) ([]Blueprint, error) {
	return []Blueprint{f.blueprint}, nil
}
func (f *fakeStore) NextVersion(context.Context, string, string) (int, error) { return 2, nil }
func (f *fakeStore) CreateVersion(_ context.Context, orgID, name string, version int, spec []byte) (Blueprint, error) {
	return Blueprint{OrgID: orgID, Name: name, Version: version, Spec: spec, Status: "draft"}, nil
}
func (f *fakeStore) Get(context.Context, string, string) (Blueprint, error) {
	return f.blueprint, nil
}
func (f *fakeStore) Publish(context.Context, string, string) (Blueprint, error) {
	return f.blueprint, nil
}
