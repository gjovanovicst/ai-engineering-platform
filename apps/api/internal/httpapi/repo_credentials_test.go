package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/repocreds"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeRepoCredentials struct {
	status    repocreds.Status
	setToken  string
	setActor  string
	deleteHit bool
	err       error
}

func (f *fakeRepoCredentials) Set(_ context.Context, _, _, actorID, token string) (repocreds.Status, error) {
	if f.err != nil {
		return repocreds.Status{}, f.err
	}
	f.setToken, f.setActor = token, actorID
	return f.status, nil
}

func (f *fakeRepoCredentials) Delete(context.Context, string, string, string) error {
	f.deleteHit = true
	return f.err
}

func (f *fakeRepoCredentials) Status(context.Context, string, string) (repocreds.Status, error) {
	return f.status, f.err
}

func (f *fakeRepoCredentials) Token(context.Context, string, string) (string, error) {
	return "", f.err
}

// credRepos reuses the webhook fake for the unused methods and only overrides
// Get, which the credential handlers call.
type credRepos struct {
	*fakeRepos
	found domain.Repo
	err   error
}

func (c credRepos) Get(context.Context, string, string, string) (domain.Repo, error) {
	return c.found, c.err
}

func credentialRequest(method, body, role string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/orgs/org-1/projects/proj-1/repos/repo-1/credential", strings.NewReader(body))
	req.SetPathValue("org", "org-1")
	req.SetPathValue("project", "proj-1")
	req.SetPathValue("repo", "repo-1")
	claims := &auth.Claims{OrgID: "org-1", Role: role, Subject: "user-1"}
	return req.WithContext(context.WithValue(req.Context(), apiKeyContextKey{}, claims))
}

func newCredentialHandlers(creds *fakeRepoCredentials, repos RepoService) *repoHandlers {
	return newRepoHandlers(repos, creds, nil, nil, nil)
}

func TestGetCredentialReturnsStatusWithoutToken(t *testing.T) {
	t.Parallel()
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	creds := &fakeRepoCredentials{status: repocreds.Status{Configured: true, Hint: "…1234", UpdatedAt: &updated}}
	h := newCredentialHandlers(creds, credRepos{found: domain.Repo{ID: "repo-1", OrgID: "org-1"}})

	rec := httptest.NewRecorder()
	h.credential(rec, credentialRequest(http.MethodGet, "", "member"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["configured"] != true || body["hint"] != "…1234" {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["token"]; ok {
		t.Fatal("the response must not contain a token")
	}
}

func TestSetCredentialRequiresAdmin(t *testing.T) {
	t.Parallel()
	creds := &fakeRepoCredentials{}
	h := newCredentialHandlers(creds, credRepos{found: domain.Repo{ID: "repo-1", OrgID: "org-1"}})

	rec := httptest.NewRecorder()
	h.setCredential(rec, credentialRequest(http.MethodPut, `{"token":"github_pat_1234567890"}`, "member"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if creds.setToken != "" {
		t.Fatal("a member must not reach the service")
	}
}

func TestSetCredentialStoresWithTheCallerAsActor(t *testing.T) {
	t.Parallel()
	creds := &fakeRepoCredentials{status: repocreds.Status{Configured: true, Hint: "…7890"}}
	h := newCredentialHandlers(creds, credRepos{found: domain.Repo{ID: "repo-1", OrgID: "org-1"}})

	rec := httptest.NewRecorder()
	h.setCredential(rec, credentialRequest(http.MethodPut, `{"token":"github_pat_1234567890"}`, "admin"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if creds.setToken != "github_pat_1234567890" || creds.setActor != "user-1" {
		t.Fatalf("service got token %q actor %q", creds.setToken, creds.setActor)
	}
}

func TestSetCredentialMapsValidationTo400(t *testing.T) {
	t.Parallel()
	creds := &fakeRepoCredentials{err: domain.ErrValidation}
	h := newCredentialHandlers(creds, credRepos{found: domain.Repo{ID: "repo-1", OrgID: "org-1"}})

	rec := httptest.NewRecorder()
	h.setCredential(rec, credentialRequest(http.MethodPut, `{"token":"x"}`, "owner"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCredentialRoutesRejectUnknownRepo(t *testing.T) {
	t.Parallel()
	h := newCredentialHandlers(&fakeRepoCredentials{}, credRepos{err: domain.ErrNotFound})

	rec := httptest.NewRecorder()
	h.credential(rec, credentialRequest(http.MethodGet, "", "owner"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteCredentialReturns204(t *testing.T) {
	t.Parallel()
	creds := &fakeRepoCredentials{}
	h := newCredentialHandlers(creds, credRepos{found: domain.Repo{ID: "repo-1", OrgID: "org-1"}})

	rec := httptest.NewRecorder()
	h.deleteCredential(rec, credentialRequest(http.MethodDelete, "", "owner"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if !creds.deleteHit {
		t.Fatal("the service was not called")
	}
}
