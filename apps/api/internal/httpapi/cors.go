package httpapi

import (
	"net/http"
	"strings"
)

// withCORS allows the browser to call the API from the web origin. The API is
// Bearer-based, so credentials are not enabled; the allowlist is explicit.
func withCORS(next http.Handler, allowed []string) http.Handler {
	allow := make(map[string]bool, len(allowed))
	for _, origin := range allowed {
		if origin = strings.TrimRight(strings.TrimSpace(origin), "/"); origin != "" {
			allow[origin] = true
		}
	}
	if len(allow) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allow[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, Idempotency-Key")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
