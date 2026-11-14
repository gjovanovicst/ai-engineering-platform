package httpapi

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
)

// CSRFConfig configures the double-submit protection (docs/12-auth.md §4.4).
type CSRFConfig struct {
	// AllowedOrigins is the allowlist of browser origins, e.g.
	// "https://app.example.com".
	AllowedOrigins []string
	// CookieName defaults to aip_csrf.
	CookieName string
	// HeaderName defaults to X-CSRF-Token.
	HeaderName string
}

// CSRF protects cookie-authenticated state-changing requests. Requests carrying
// a Bearer token are exempt: they have no ambient credentials to abuse.
func CSRF(cfg CSRFConfig) func(http.Handler) http.Handler {
	cookieName := cfg.CookieName
	if cookieName == "" {
		cookieName = "aip_csrf"
	}
	headerName := cfg.HeaderName
	if headerName == "" {
		headerName = "X-CSRF-Token"
	}
	origins := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		origins[strings.TrimRight(o, "/")] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) || hasBearer(r) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				csrfFailure(w, "missing csrf cookie")
				return
			}
			header := r.Header.Get(headerName)
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				csrfFailure(w, "csrf token mismatch")
				return
			}
			if !originAllowed(r, origins) {
				csrfFailure(w, "origin not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

func hasBearer(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	return len(h) > 7 && strings.EqualFold(h[:7], "Bearer ")
}

func originAllowed(r *http.Request, allowed map[string]bool) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Fall back to Referer for older clients, but never accept a request
		// with neither header: browsers always send Origin on unsafe methods.
		ref := r.Header.Get("Referer")
		if ref == "" {
			return false
		}
		u, err := url.Parse(ref)
		if err != nil {
			return false
		}
		origin = u.Scheme + "://" + u.Host
	}
	return allowed[strings.TrimRight(origin, "/")]
}

func csrfFailure(w http.ResponseWriter, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"type":"https://api.aip.dev/errors/forbidden","title":"Forbidden","status":403,"code":"csrf_failed","detail":"` + detail + `"}`))
}
