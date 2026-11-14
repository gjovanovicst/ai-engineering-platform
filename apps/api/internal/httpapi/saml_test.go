package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	crewsaml "github.com/crewjam/saml"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/replay"
)

type fakeSAMLSP struct {
	assertion  *crewsaml.Assertion
	parseErr   error
	panicValue any
}

func (f fakeSAMLSP) Metadata() ([]byte, error) { return []byte("<xml/>"), nil }
func (f fakeSAMLSP) LoginURL() (string, string, error) {
	return "https://idp.example/sso", "req-1", nil
}
func (f fakeSAMLSP) Parse(*http.Request, string) (*crewsaml.Assertion, error) {
	if f.panicValue != nil {
		panic(f.panicValue)
	}
	return f.assertion, f.parseErr
}
func (f fakeSAMLSP) WebURL() string { return "http://web.test" }
func (f fakeSAMLSP) PostLogout(string) ([]byte, error) {
	return []byte("<form/>"), nil
}

type fakeSAMLAccounts struct {
	identity auth.OIDCIdentity
	calls    int
}

func (f *fakeSAMLAccounts) OIDCLogin(_ context.Context, id auth.OIDCIdentity, _ auth.Platform) (auth.TokenPair, error) {
	f.identity = id
	f.calls++
	return auth.TokenPair{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func acsRequest(relay string) *http.Request {
	form := url.Values{}
	if relay != "" {
		form.Set("RelayState", relay)
	}
	req := httptest.NewRequest(http.MethodPost, "/saml/acs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func newSAMLTest(sp SAMLSP, accounts SAMLAccounts) *samlHandlers {
	return &samlHandlers{svc: sp, accounts: accounts, replay: replay.NewMemory(), log: slog.New(slog.DiscardHandler)}
}

func TestSAMLAcsRequiresRelayState(t *testing.T) {
	h := newSAMLTest(fakeSAMLSP{}, &fakeSAMLAccounts{})
	rec := httptest.NewRecorder()
	h.acs(rec, acsRequest(""))
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/login?error=saml") {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSAMLAcsRejectsParseError(t *testing.T) {
	h := newSAMLTest(fakeSAMLSP{parseErr: errors.New("bad signature")}, &fakeSAMLAccounts{})
	rec := httptest.NewRecorder()
	h.acs(rec, acsRequest("req-1"))
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/login?error=saml") {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSAMLAcsRejectsReplay(t *testing.T) {
	assertion := &crewsaml.Assertion{ID: "assert-1", Subject: &crewsaml.Subject{NameID: &crewsaml.NameID{Value: "user@example.com"}}}
	accounts := &fakeSAMLAccounts{}
	h := newSAMLTest(fakeSAMLSP{assertion: assertion}, accounts)
	if seen, _ := h.replay.Seen(context.Background(), "assert-1", assertionExpiry(assertion)); seen {
		t.Fatal("unexpected replay")
	}
	rec := httptest.NewRecorder()
	h.acs(rec, acsRequest("req-1"))
	if strings.Contains(rec.Header().Get("Location"), "/auth/callback") {
		t.Fatalf("replay reached login: %q", rec.Header().Get("Location"))
	}
	if accounts.calls != 0 {
		t.Fatalf("accounts called %d times", accounts.calls)
	}
}

func TestSAMLAcsSuccessRedirectsWithTokens(t *testing.T) {
	assertion := &crewsaml.Assertion{ID: "assert-2", Subject: &crewsaml.Subject{NameID: &crewsaml.NameID{Value: "user@example.com"}}}
	accounts := &fakeSAMLAccounts{}
	h := newSAMLTest(fakeSAMLSP{assertion: assertion}, accounts)
	rec := httptest.NewRecorder()
	h.acs(rec, acsRequest("req-1"))
	location := rec.Header().Get("Location")
	if rec.Code != http.StatusFound || !strings.HasPrefix(location, "http://web.test/auth/callback#access=access") {
		t.Fatalf("status=%d location=%q", rec.Code, location)
	}
	if accounts.identity.Email != "user@example.com" || accounts.identity.Provider != "saml" {
		t.Fatalf("identity=%+v", accounts.identity)
	}
}

func TestSAMLHandlerRoleFor(t *testing.T) {
	h := &samlHandlers{roleMap: map[string]string{"aip-admins": "admin", "aip-viewers": "viewer"}, adminGroups: map[string]bool{"legacy-admins": true}}
	if got := h.roleFor([]string{"aip-viewers"}); got != "viewer" {
		t.Fatalf("got %q", got)
	}
	if got := h.roleFor([]string{"legacy-admins"}); got != "admin" {
		t.Fatalf("got %q", got)
	}
	if got := h.roleFor([]string{"unknown"}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSAMLAcsRecoversParserPanic(t *testing.T) {
	h := newSAMLTest(fakeSAMLSP{panicValue: "bad assertion"}, &fakeSAMLAccounts{})
	rec := httptest.NewRecorder()
	h.acs(rec, acsRequest("req-1"))
	if rec.Code != http.StatusFound || !strings.Contains(rec.Header().Get("Location"), "/login?error=saml") {
		t.Fatalf("status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
}
