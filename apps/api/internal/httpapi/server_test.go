package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newTestRouter(db Pinger) http.Handler {
	return NewRouter(Deps{
		Version:   "test",
		StartedAt: time.Now(),
		Log:       slog.New(slog.DiscardHandler),
		DB:        db,
	})
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(fakePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyzReady(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(fakePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyzDatabaseDown(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(fakePinger{err: errors.New("connection refused")}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestReadyzWithoutDatabase(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestUnknownRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(fakePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter(fakePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	for header, want := range map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "no-referrer",
		"Cross-Origin-Opener-Policy": "same-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}
