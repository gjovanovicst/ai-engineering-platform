package git

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiteaCreateMergeAndWebhook(t *testing.T) {
	var createdPath, mergePath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls":
			createdPath = r.URL.Path
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["head"] != "aip/x" || body["base"] != "main" {
				t.Errorf("body = %v", body)
			}
			_, _ = w.Write([]byte(`{"number":3,"html_url":"https://gitea.test/o/r/pulls/3","state":"open"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/3/merge":
			mergePath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	g := NewGitea("tok", nil)
	g.BaseURL = server.URL
	g.HTTP = server.Client()

	pr, err := g.CreatePullRequest(context.Background(), CreateRequest{Repo: "o/r", Title: "t", Head: "aip/x", Base: "main"})
	if err != nil || pr.Number != 3 || pr.URL == "" {
		t.Fatalf("pr=%+v err=%v", pr, err)
	}
	if err := g.MergePullRequest(context.Background(), "o/r", 3, "title"); err != nil {
		t.Fatal(err)
	}
	if createdPath == "" || mergePath == "" {
		t.Fatalf("paths = %q %q", createdPath, mergePath)
	}
}

func TestGiteaWebhookVerifiesAndNormalises(t *testing.T) {
	secret := []byte("s3cret")
	g := NewGitea("tok", secret)
	body := []byte(`{"action":"closed","number":7,"pull_request":{"merged":true,"html_url":"u"},"repository":{"full_name":"o/r"}}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitea/repo", nil)
	req.Header.Set("X-Gitea-Signature", sig)
	req.Header.Set("X-Gitea-Event", "pull_request")
	event, err := g.ParseWebhook(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if !event.Merged || event.Number != 7 || event.Repo != "o/r" {
		t.Fatalf("event = %+v", event)
	}

	req.Header.Set("X-Gitea-Signature", "bad")
	if _, err := g.ParseWebhook(req, body); err == nil {
		t.Fatal("bad signature must fail")
	}
}
