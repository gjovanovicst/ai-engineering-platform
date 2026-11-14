package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/ratelimit"
)

type fakeAuth struct {
	registerErr error
	loginErr    error
	verifyErr   error
	resetErr    error
	pair        auth.TokenPair
}

func (f *fakeAuth) Register(context.Context, auth.RegisterInput) (auth.RegisterResult, error) {
	if f.registerErr != nil {
		return auth.RegisterResult{}, f.registerErr
	}
	return auth.RegisterResult{UserID: "user-1", OrgSlug: "acme"}, nil
}

func (f *fakeAuth) VerifyEmail(context.Context, string) error { return f.verifyErr }

func (f *fakeAuth) Login(context.Context, auth.LoginInput) (auth.TokenPair, error) {
	if f.loginErr != nil {
		return auth.TokenPair{}, f.loginErr
	}
	return f.pair, nil
}

func (f *fakeAuth) Refresh(context.Context, string, auth.Platform) (auth.TokenPair, error) {
	return f.pair, nil
}

func (f *fakeAuth) Logout(context.Context, string) error    { return nil }
func (f *fakeAuth) LogoutAll(context.Context, string) error { return nil }

func (f *fakeAuth) RequestPasswordReset(context.Context, string, string) error { return f.resetErr }

func (f *fakeAuth) ResetPassword(context.Context, string, string) error { return f.resetErr }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func testIssuer(t *testing.T) *auth.Issuer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	issuer, err := auth.NewIssuer(pemKey, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}

func authRouter(t *testing.T, svc AccountService, limits RateLimits) (http.Handler, *auth.Issuer) {
	t.Helper()
	issuer := testIssuer(t)
	return NewRouter(Deps{
		Version: "test", StartedAt: time.Now(),
		Log:    discardLogger(),
		Auth:   svc,
		Issuer: issuer,
		Limits: limits,
	}), issuer
}

func TestRegisterHTTP(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"a@acme.io","name":"A","password":"correct horse battery","org_name":"Acme"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "org_slug") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestRegisterConflictIsNotDisclosed(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{registerErr: domain.ErrConflict}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"a@acme.io","name":"A","password":"correct horse battery"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestRegisterValidationHTTP(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{registerErr: auth.ErrWeakPassword}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"a@acme.io","name":"A","password":"short"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLoginHTTP(t *testing.T) {
	pair := auth.TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900}
	router, _ := authRouter(t, &fakeAuth{pair: pair}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"a@acme.io","password":"correct horse battery"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"access_token":"access"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestLoginInvalidCredentialsHTTP(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{loginErr: auth.ErrInvalidCredentials}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"a@acme.io","password":"wrong"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "not found") {
		t.Fatal("response leaks account existence")
	}
}

func TestLoginRateLimited(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{loginErr: auth.ErrInvalidCredentials},
		RateLimits{Login: ratelimit.New(1, time.Minute)})
	body := `{"email":"a@acme.io","password":"wrong"}`
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		router.ServeHTTP(rec, req)
		want := http.StatusUnauthorized
		if i == 1 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", i+1, rec.Code, want)
		}
	}
}

func TestMeRequiresToken(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{}, RateLimits{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestMeWithValidToken(t *testing.T) {
	router, issuer := authRouter(t, &fakeAuth{}, RateLimits{})
	raw, _, err := issuer.Issue("user-1", auth.Claims{OrgID: "org-1", Role: "admin", SessionID: "sess-1"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"org_id":"org-1"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestVerifyEmailGone(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{verifyErr: auth.ErrInvalidToken}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"x"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
}

func TestResetRequestAlwaysAccepted(t *testing.T) {
	router, _ := authRouter(t, &fakeAuth{}, RateLimits{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/reset-request",
		strings.NewReader(`{"email":"nobody@acme.io"}`))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}
