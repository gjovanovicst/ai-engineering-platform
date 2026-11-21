package git

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeLookup struct {
	repo domain.Repo
	err  error
}

func (f fakeLookup) ByProject(context.Context, string, string) (domain.Repo, error) {
	return f.repo, f.err
}

type fakeRecorder struct{ pr domain.PullRequest }

func (f *fakeRecorder) RecordPullRequest(_ context.Context, pr domain.PullRequest) (domain.PullRequest, error) {
	pr.ID = "pr-1"
	f.pr = pr
	return pr, nil
}

type fakePusher struct{ req PushRequest }

func (f *fakePusher) Push(_ context.Context, _ []byte, req PushRequest) error {
	f.req = req
	return nil
}

func TestPublisherPushesAndOpensPR(t *testing.T) {
	pusher := &fakePusher{}
	recorder := &fakeRecorder{}
	var created CreateRequest
	p := &Publisher{
		Repos: fakeLookup{repo: domain.Repo{
			ID: "repo-1", OrgID: "o", ProjectID: "p", Provider: domain.ProviderGitHub,
			Name: "o/r", DefaultBranch: "main",
		}},
		Recorder: recorder,
		Pusher:   pusher,
		Providers: func(provider, token, secret string) Provider {
			return fakeProvider{create: func(req CreateRequest) (PullRequest, error) {
				created = req
				return PullRequest{Number: 7, URL: "https://github.com/o/r/pull/7"}, nil
			}}
		},
		Tokens: map[string]string{"github": "tok"},
	}

	pr, err := p.Publish(context.Background(), domain.Run{ID: "abcdef12-3456", OrgID: "o", ProjectID: "p", TaskID: "t1"}, []byte("tar"))
	if err != nil {
		t.Fatal(err)
	}
	if pr.ID != "pr-1" {
		t.Fatalf("returned pr = %+v", pr)
	}
	if !strings.Contains(pusher.req.RemoteURL, "x-access-token:tok@github.com/o/r.git") {
		t.Fatalf("remote = %q", pusher.req.RemoteURL)
	}
	if pusher.req.Branch != "aip/abcdef12" {
		t.Fatalf("branch = %q", pusher.req.Branch)
	}
	if created.Head != "aip/abcdef12" || created.Base != "main" {
		t.Fatalf("create = %+v", created)
	}
	if recorder.pr.Number != 7 || recorder.pr.TaskID != "t1" || recorder.pr.URL == "" {
		t.Fatalf("recorded = %+v", recorder.pr)
	}
}

type fakeProvider struct {
	create func(CreateRequest) (PullRequest, error)
}

func (f fakeProvider) Name() string { return "fake" }
func (f fakeProvider) CreatePullRequest(_ context.Context, req CreateRequest) (PullRequest, error) {
	return f.create(req)
}
func (f fakeProvider) MergePullRequest(context.Context, string, int, string) error { return nil }
func (f fakeProvider) ParseWebhook(_ *http.Request, _ []byte) (WebhookEvent, error) {
	return WebhookEvent{}, nil
}

type fakeTaskLookup struct{ task domain.Task }

func (f fakeTaskLookup) Get(context.Context, string, string, string) (domain.Task, error) {
	return f.task, nil
}

func TestPublisherBodyUsesTaskAndPlan(t *testing.T) {
	var created CreateRequest
	p := &Publisher{
		Repos: fakeLookup{repo: domain.Repo{
			ID: "repo-1", OrgID: "o", ProjectID: "p", Provider: domain.ProviderGitHub,
			Name: "o/r", DefaultBranch: "main",
		}},
		Recorder: &fakeRecorder{},
		Pusher:   &fakePusher{},
		Tasks: fakeTaskLookup{task: domain.Task{
			Ref: "T-9", Title: "Add a customer page", Description: "A form.",
			Acceptance: []domain.AcceptanceCriterion{{Given: "g", When: "w", Then: "t"}},
		}},
		WebURL: "http://web.test",
		Providers: func(provider, token, secret string) Provider {
			return fakeProvider{create: func(req CreateRequest) (PullRequest, error) {
				created = req
				return PullRequest{Number: 1, URL: "https://github.com/o/r/pull/1"}, nil
			}}
		},
		Tokens: map[string]string{"github": "tok"},
	}
	run := domain.Run{
		ID: "abcdef12-3456", OrgID: "o", ProjectID: "p", TaskID: "t1",
		Plan: []byte(`{"steps":["Do it"],"files":["main.go"],"risks":["low"]}`),
		Diff: []byte("diff --git a/main.go b/main.go\n"),
	}
	if _, err := p.Publish(context.Background(), run, []byte("tar")); err != nil {
		t.Fatal(err)
	}
	if created.Title != "T-9: Add a customer page" {
		t.Fatalf("title = %q", created.Title)
	}
	for _, want := range []string{"## How tested", "Do it", "main.go", "1 file(s)", "http://web.test/projects/p/runs/abcdef12-3456", "Given g, when w, then t"} {
		if !strings.Contains(created.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, created.Body)
		}
	}
}

func TestPRBodyRendersAcceptanceEvidence(t *testing.T) {
	task := domain.Task{Ref: "T-1", Title: "Paginate", Acceptance: []domain.AcceptanceCriterion{{Given: "a list", When: "next", Then: "page two"}}}
	run := domain.Run{AcceptanceProof: []byte(`{"evidence":[{"criterion":"AC1","kind":"automated","test":"TestPager"},{"criterion":"AC2","kind":"manual","reason":"verified by QA"}]}`)}
	body := prBody(task, run, "http://web")
	for _, want := range []string{"## Acceptance evidence", "TestPager", "manual: verified by QA", "Given a list, when next, then page two"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestPublisherTokenPrecedence(t *testing.T) {
	t.Parallel()
	repoToken := func(value string) func(context.Context, domain.Repo) (string, error) {
		return func(context.Context, domain.Repo) (string, error) { return value, nil }
	}
	orgToken := func(value string) func(context.Context, string, string) (string, error) {
		return func(context.Context, string, string) (string, error) { return value, nil }
	}
	providerToken := func(value string) func(context.Context, string) (string, error) {
		return func(context.Context, string) (string, error) { return value, nil }
	}
	github := domain.Repo{ID: "repo-1", OrgID: "org-1", Provider: domain.ProviderGitHub}
	gitlab := domain.Repo{ID: "repo-2", OrgID: "org-1", Provider: domain.ProviderGitLab}

	cases := []struct {
		name   string
		p      Publisher
		repo   domain.Repo
		want   string
		absent bool
	}{
		{
			name: "repository credential wins",
			p: Publisher{
				RepoTokenFunc: repoToken("repo-token"), OrgTokenFunc: orgToken("app-token"),
				TokenFunc: providerToken("global-app"), Tokens: map[string]string{"github": "static"},
			},
			repo: github, want: "repo-token",
		},
		{
			name: "app token when the repo has none",
			p: Publisher{
				RepoTokenFunc: repoToken(""), OrgTokenFunc: orgToken("app-token"),
				Tokens: map[string]string{"github": "static"},
			},
			repo: github, want: "app-token",
		},
		{
			name: "global token when neither repo nor app resolves",
			p: Publisher{
				RepoTokenFunc: repoToken(""), OrgTokenFunc: orgToken(""),
				Tokens: map[string]string{"github": "static"},
			},
			repo: github, want: "static",
		},
		{
			name: "gitlab skips the GitHub App and uses the static token",
			p: Publisher{
				RepoTokenFunc: repoToken(""), OrgTokenFunc: orgToken("app-token"),
				Tokens: map[string]string{"gitlab": "glpat-static"},
			},
			repo: gitlab, want: "glpat-static",
		},
		{
			name:   "no source configured is an error",
			p:      Publisher{},
			repo:   github,
			absent: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.p.tokenFor(context.Background(), tc.repo.OrgID, tc.repo)
			if tc.absent {
				if err == nil {
					t.Fatalf("token = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("tokenFor: %v", err)
			}
			if got != tc.want {
				t.Fatalf("token = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPRBodyHasTheRequiredSections(t *testing.T) {
	t.Parallel()
	body := prBody(domain.Task{Ref: "T-1", Title: "Add a health endpoint", Description: "the description"}, domain.Run{}, "https://app.test")
	for _, section := range []string{"## What", "## Why", "## How tested", "## Risk and rollback", "## Screenshot"} {
		if !strings.Contains(body, section) {
			t.Errorf("the PR body misses the section %q", section)
		}
	}
}
