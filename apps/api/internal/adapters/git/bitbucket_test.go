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

func TestBitbucketCreateMergeAndWebhook(t *testing.T) {
	var create, merge bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic dXNlcjphcHA=" { // user:app
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repositories/o/r/pullrequests":
			create = true
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["source"].(map[string]any)["branch"].(map[string]any)["name"] != "aip/x" {
				t.Errorf("body = %v", body)
			}
			_, _ = w.Write([]byte(`{"id":5,"state":"OPEN","links":{"html":{"href":"https://bitbucket.org/o/r/pull-requests/5"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repositories/o/r/pullrequests/5/merge":
			merge = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	b := NewBitbucket("user:app", nil)
	b.BaseURL = server.URL
	b.HTTP = server.Client()
	pr, err := b.CreatePullRequest(context.Background(), CreateRequest{Repo: "o/r", Title: "t", Head: "aip/x", Base: "main"})
	if err != nil || pr.Number != 5 || pr.URL == "" {
		t.Fatalf("pr=%+v err=%v", pr, err)
	}
	if err := b.MergePullRequest(context.Background(), "o/r", 5, "t"); err != nil {
		t.Fatal(err)
	}
	if !create || !merge {
		t.Fatalf("create=%v merge=%v", create, merge)
	}
}

func TestBitbucketWebhook(t *testing.T) {
	secret := []byte("s3cret")
	b := NewBitbucket("user:app", secret)
	body := []byte(`{"pullrequest":{"id":9,"state":"MERGED"},"repository":{"full_name":"o/r"}}`)
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/bitbucket/r", nil)
	req.Header.Set("X-Hub-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Event-Key", "pullrequest:fulfilled")
	event, err := b.ParseWebhook(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if !event.Merged || event.Number != 9 || event.Repo != "o/r" {
		t.Fatalf("event = %+v", event)
	}
	req.Header.Set("X-Hub-Signature", "sha256=bad")
	if _, err := b.ParseWebhook(req, body); err == nil {
		t.Fatal("bad signature must fail")
	}
}
