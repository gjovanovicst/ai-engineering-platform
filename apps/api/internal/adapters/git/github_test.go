package git

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubCreatePullRequest(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = decodeJSON(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":42,"html_url":"https://github.com/o/r/pull/42","state":"open"}`))
	}))
	defer server.Close()

	p := NewGitHub("tok-1", "secret")
	p.BaseURL = server.URL
	p.Client = server.Client()

	pr, err := p.CreatePullRequest(context.Background(), CreateRequest{
		Repo: "o/r", Title: "Add SSO", Body: "plan", Head: "aip/T-1", Base: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 42 || pr.URL == "" || pr.State != "open" {
		t.Fatalf("pr = %+v", pr)
	}
	if gotPath != "/repos/o/r/pulls" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody["head"] != "aip/T-1" || gotBody["base"] != "main" {
		t.Fatalf("body = %v", gotBody)
	}
}

func TestGitHubParseWebhook(t *testing.T) {
	secret := []byte("s3cret")
	body := []byte(`{"action":"closed","number":7,"pull_request":{"merged":true,"state":"closed"},"repository":{"full_name":"o/r"}}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	p := NewGitHub("tok", string(secret))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", nil)
	req.Header.Set("X-Hub-Signature-256", signature)

	event, err := p.ParseWebhook(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if !event.Merged || event.Number != 7 || event.Repo != "o/r" {
		t.Fatalf("event = %+v", event)
	}

	bad := httptest.NewRequest(http.MethodPost, "/webhooks/github", nil)
	bad.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	if _, err := p.ParseWebhook(bad, body); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("err = %v, want signature error", err)
	}
}

func decodeJSON(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

func TestGitHubMergePullRequest(t *testing.T) {
	var method, path, auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"merged":true}`))
	}))
	defer server.Close()

	p := NewGitHub("tok", "")
	p.BaseURL = server.URL
	p.Client = server.Client()
	if err := p.MergePullRequest(context.Background(), "o/r", 42, "aip: merge"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/repos/o/r/pulls/42/merge" {
		t.Fatalf("request = %s %s", method, path)
	}
	if auth != "Bearer tok" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestGitHubProtectBranch(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	gh := NewGitHub("tok", "")
	gh.BaseURL = srv.URL
	if err := gh.ProtectBranch(context.Background(), "o/r", "main", []string{"test"}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/o/r/branches/main/protection" || gotAuth != "Bearer tok" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
	if gotBody["enforce_admins"] != false {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestGitHubRebasePullRequest(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	gh := NewGitHub("tok", "")
	gh.BaseURL = srv.URL
	if err := gh.RebasePullRequest(context.Background(), "o/r", 7); err != nil {
		t.Fatal(err)
	}
	if path != "/repos/o/r/pulls/7/update-branch" {
		t.Fatalf("path = %q", path)
	}
}

func TestGitHubFindOpenPullRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("head") != "o:branch" {
			t.Errorf("head = %q", r.URL.Query().Get("head"))
		}
		_, _ = w.Write([]byte(`[{"number":12,"html_url":"http://pr/12"}]`))
	}))
	defer srv.Close()
	gh := NewGitHub("tok", "")
	gh.BaseURL = srv.URL
	number, url, found, err := gh.FindOpenPullRequest(context.Background(), "o/r", "branch")
	if err != nil || !found || number != 12 || url != "http://pr/12" {
		t.Fatalf("number=%d url=%q found=%v err=%v", number, url, found, err)
	}
}

func TestGitHubPullRequestDiff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github.v3.diff" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte("--- a/x\n+++ b/x\n"))
	}))
	defer srv.Close()
	gh := NewGitHub("tok", "")
	gh.BaseURL = srv.URL
	diff, err := gh.PullRequestDiff(context.Background(), "o/r", 3)
	if err != nil || diff == "" {
		t.Fatalf("diff=%q err=%v", diff, err)
	}
}

func TestGitHubRebaseOmitsExpectedHeadSha(t *testing.T) {
	t.Parallel()
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message":"Updating pull request branch"}`))
	}))
	defer server.Close()

	p := NewGitHub("tok-1", "")
	p.BaseURL = server.URL
	p.Client = server.Client()
	if err := p.RebasePullRequest(context.Background(), "o/r", 7); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/o/r/pulls/7/update-branch" {
		t.Fatalf("path = %q", gotPath)
	}
	if strings.Contains(gotBody, "expected_head_sha") {
		t.Fatalf("must not send an empty expected_head_sha, body = %s", gotBody)
	}
}

func TestGitHubRebaseTreatsUpToDateAsSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Unable to update pull request branch: branch is already up to date"}`))
	}))
	defer server.Close()

	p := NewGitHub("tok-1", "")
	p.BaseURL = server.URL
	p.Client = server.Client()
	if err := p.RebasePullRequest(context.Background(), "o/r", 7); err != nil {
		t.Fatalf("an up-to-date branch must not fail the queue: %v", err)
	}
}

func TestGitHubRebaseReportsRealErrors(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"expected head sha didn't match current head ref."}`))
	}))
	defer server.Close()

	p := NewGitHub("tok-1", "")
	p.BaseURL = server.URL
	p.Client = server.Client()
	if err := p.RebasePullRequest(context.Background(), "o/r", 7); err == nil {
		t.Fatal("a real rebase error must be returned")
	}
}
