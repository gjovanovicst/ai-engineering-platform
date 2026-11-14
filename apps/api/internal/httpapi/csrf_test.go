package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func csrfHandler() http.Handler {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return CSRF(CSRFConfig{AllowedOrigins: []string{"https://app.test"}})(next)
}

func TestCSRFSafeMethodPasses(t *testing.T) {
	rec := httptest.NewRecorder()
	csrfHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestCSRFBearerIsExempt(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	csrfHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestCSRFValidDoubleSubmit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "aip_csrf", Value: "abc123"})
	req.Header.Set("X-CSRF-Token", "abc123")
	req.Header.Set("Origin", "https://app.test")
	rec := httptest.NewRecorder()
	csrfHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestCSRFFailures(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		header string
		origin string
	}{
		{"missing cookie", "", "abc", "https://app.test"},
		{"missing header", "abc", "", "https://app.test"},
		{"mismatched token", "abc", "xyz", "https://app.test"},
		{"unknown origin", "abc", "abc", "https://evil.test"},
		{"no origin or referer", "abc", "abc", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/x", nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "aip_csrf", Value: tc.cookie})
			}
			if tc.header != "" {
				req.Header.Set("X-CSRF-Token", tc.header)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			csrfHandler().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "csrf_failed") {
				t.Fatalf("body = %s, want csrf_failed", rec.Body.String())
			}
		})
	}
}

func TestCSRFRefererFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "aip_csrf", Value: "abc"})
	req.Header.Set("X-CSRF-Token", "abc")
	req.Header.Set("Referer", "https://app.test/settings")
	rec := httptest.NewRecorder()
	csrfHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
