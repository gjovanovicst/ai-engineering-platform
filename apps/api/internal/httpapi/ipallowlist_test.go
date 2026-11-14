package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIPAllowlist(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	h := withIPAllowlist(ok, []netip.Prefix{prefix})
	req := httptest.NewRequest("GET", "/api/v1/orgs/x/projects", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed code = %d", rec.Code)
	}
	req.RemoteAddr = "192.168.1.1:1234"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("blocked code = %d", rec.Code)
	}
	req = httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health code = %d", rec.Code)
	}
}
