package git

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubParsesCheckRun(t *testing.T) {
	provider := NewGitHub("", "")
	body := `{
		"action":"completed",
		"check_run":{"name":"test (go)","status":"completed","conclusion":"success",
			"head_sha":"abc123","output":{"summary":"ok"},
			"pull_requests":[{"number":7}]},
		"repository":{"full_name":"acme/repo"}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/x", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "check_run")
	event, err := provider.ParseWebhook(req, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != "check_run" || event.Number != 7 || event.CheckStatus != "passed" {
		t.Fatalf("event = %+v", event)
	}
	if event.CheckName != "test (go)" || event.CommitSHA != "abc123" {
		t.Fatalf("event = %+v", event)
	}
}

func TestGitHubMapsFailedConclusion(t *testing.T) {
	provider := NewGitHub("", "")
	body := `{"check_run":{"name":"lint","status":"completed","conclusion":"failure","pull_requests":[{"number":1}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/x", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "check_run")
	event, err := provider.ParseWebhook(req, []byte(body))
	if err != nil || event.CheckStatus != "failed" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
}
