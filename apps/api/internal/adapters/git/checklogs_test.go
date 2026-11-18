package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeRepoGetter struct{ repo domain.Repo }

func (f fakeRepoGetter) Get(context.Context, string, string, string) (domain.Repo, error) {
	return f.repo, nil
}

func TestParseJobID(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"https://github.com/acme/demo/actions/runs/123/job/456": "456",
		"https://github.com/acme/demo/checks/1":                 "",
		"":                                                      "",
	}
	for url, want := range cases {
		if got := parseJobID(url); got != want {
			t.Fatalf("parseJobID(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestCheckLogsFetch(t *testing.T) {
	t.Parallel()
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte("job failed: go test ./...\n--- FAIL: TestX\n"))
	}))
	defer server.Close()
	logs := &CheckLogs{
		Repos:  fakeRepoGetter{repo: domain.Repo{Provider: domain.ProviderGitHub, Name: "acme/demo"}},
		Tokens: map[string]string{"github": "tok"},
		APIURL: server.URL,
		HTTP:   server.Client(),
	}
	body, err := logs.Fetch(context.Background(), "o", "p", "r",
		"https://github.com/acme/demo/actions/runs/1/job/42")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/acme/demo/actions/jobs/42/logs" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if len(body) == 0 || body[:3] != "job" {
		t.Fatalf("body = %q", body)
	}
}

func TestCheckLogsRejectsOtherProviders(t *testing.T) {
	t.Parallel()
	logs := &CheckLogs{Repos: fakeRepoGetter{repo: domain.Repo{Provider: domain.ProviderGitLab, Name: "acme/demo"}}}
	if _, err := logs.Fetch(context.Background(), "o", "p", "r", "https://gitlab.com/x/-/jobs/1"); err == nil {
		t.Fatal("expected an error for a non-GitHub provider")
	}
}
