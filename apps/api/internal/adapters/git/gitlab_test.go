package git

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGitLabCreateMergeRequest(t *testing.T) {
	var gotPath, gotToken string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"iid":9,"web_url":"https://gitlab.com/g/p/-/merge_requests/9","state":"opened"}`))
	}))
	defer server.Close()

	p := NewGitLab("gl-token", "hook-secret")
	p.BaseURL = server.URL
	p.Client = server.Client()

	mr, err := p.CreatePullRequest(context.Background(), CreateRequest{
		Repo: "group/project", Title: "Add SSO", Body: "plan", Head: "aip/T-1", Base: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mr.Number != 9 || mr.State != "opened" {
		t.Fatalf("mr = %+v", mr)
	}
	if gotPath != "/api/v4/projects/group%2Fproject/merge_requests" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotToken != "gl-token" {
		t.Fatalf("token = %q", gotToken)
	}
	if gotBody["source_branch"] != "aip/T-1" || gotBody["target_branch"] != "main" {
		t.Fatalf("body = %v", gotBody)
	}
}

func TestGitLabParseWebhook(t *testing.T) {
	body := []byte(`{"object_kind":"merge_request","project":{"path_with_namespace":"group/project"},"object_attributes":{"iid":9,"action":"merge","state":"merged"}}`)
	p := NewGitLab("tok", "hook-secret")

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", nil)
	req.Header.Set("X-Gitlab-Token", "hook-secret")
	event, err := p.ParseWebhook(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if !event.Merged || event.Number != 9 || event.Repo != "group/project" {
		t.Fatalf("event = %+v", event)
	}

	bad := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab", nil)
	bad.Header.Set("X-Gitlab-Token", "wrong")
	if _, err := p.ParseWebhook(bad, body); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("err = %v, want token error", err)
	}
}

func TestGitLabMergeRequest(t *testing.T) {
	var method, path, token string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, token = r.Method, r.URL.EscapedPath(), r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"state":"merged"}`))
	}))
	defer server.Close()

	p := NewGitLab("gl-token", "")
	p.BaseURL = server.URL
	p.Client = server.Client()
	if err := p.MergePullRequest(context.Background(), "group/project", 7, "aip: merge"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPut || path != "/api/v4/projects/group%2Fproject/merge_requests/7/merge" {
		t.Fatalf("request = %s %s", method, path)
	}
	if token != "gl-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestGitLabProtectBranch(t *testing.T) {
	var path, tok string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, tok = r.URL.Path, r.Header.Get("PRIVATE-TOKEN")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	gl := NewGitLab("tok", "")
	gl.BaseURL = srv.URL
	if err := gl.ProtectBranch(context.Background(), "group/repo", "main", nil); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v4/projects/group/repo/protected_branches" || tok != "tok" {
		t.Fatalf("path=%q tok=%q", path, tok)
	}
}

func TestGitLabRebaseSucceeds(t *testing.T) {
	var putPath string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putPath = r.URL.EscapedPath()
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"rebase_in_progress":true}`))
			return
		}
		calls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"iid":7,"rebase_in_progress":false,"has_conflicts":false,"merge_status":"can_be_merged"}`))
	}))
	defer srv.Close()

	gl := NewGitLab("tok", "")
	gl.BaseURL = srv.URL
	gl.Client = srv.Client()
	gl.RebasePoll = time.Millisecond
	gl.RebaseTimeout = time.Second
	if err := gl.RebasePullRequest(context.Background(), "group/project", 7); err != nil {
		t.Fatal(err)
	}
	if putPath != "/api/v4/projects/group%2Fproject/merge_requests/7/rebase" {
		t.Fatalf("path = %q", putPath)
	}
	if calls == 0 {
		t.Fatal("expected a status poll")
	}
}

func TestGitLabRebaseConflict(t *testing.T) {
	gets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"rebase_in_progress":true}`))
			return
		}
		gets++
		if gets == 1 {
			_, _ = w.Write([]byte(`{"iid":7,"rebase_in_progress":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"iid":7,"rebase_in_progress":false,"has_conflicts":true,"merge_status":"cannot_be_merged","merge_error":"rebase failed"}`))
	}))
	defer srv.Close()

	gl := NewGitLab("tok", "")
	gl.BaseURL = srv.URL
	gl.Client = srv.Client()
	gl.RebasePoll = time.Millisecond
	gl.RebaseTimeout = time.Second
	err := gl.RebasePullRequest(context.Background(), "group/project", 7)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("err = %v, want conflict", err)
	}
	if gets < 2 {
		t.Fatalf("polls = %d, want >= 2", gets)
	}
}

func TestGitLabRebaseHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"rebase is not allowed"}`))
	}))
	defer srv.Close()

	gl := NewGitLab("tok", "")
	gl.BaseURL = srv.URL
	gl.Client = srv.Client()
	err := gl.RebasePullRequest(context.Background(), "group/project", 7)
	if err == nil || !strings.Contains(err.Error(), "rebase") {
		t.Fatalf("err = %v", err)
	}
}
