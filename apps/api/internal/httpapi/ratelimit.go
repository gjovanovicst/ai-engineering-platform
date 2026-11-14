package httpapi

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// orgRateLimiter is a fixed one-minute window per organisation. It is
// intentionally in-memory: the limit protects the process, not a global quota.
type orgRateLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]*orgWindow
}

type orgWindow struct {
	start time.Time
	count int
}

func newOrgRateLimiter(limit int) *orgRateLimiter {
	return &orgRateLimiter{limit: limit, windows: make(map[string]*orgWindow)}
}

func (l *orgRateLimiter) allow(org string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[org]
	if !ok || now.Sub(w.start) >= time.Minute {
		l.windows[org] = &orgWindow{start: now, count: 1}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// withOrgRateLimit applies the per-org limit to authenticated API calls. Auth
// endpoints and webhooks have no org claim and are skipped.
func withOrgRateLimit(next http.Handler, issuer *auth.Issuer, limit int) http.Handler {
	if limit <= 0 || issuer == nil {
		return next
	}
	limiter := newOrgRateLimiter(limit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			next.ServeHTTP(w, r)
			return
		}
		claims, ok := bearerClaims(issuer, r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if !limiter.allow(claims.OrgID, time.Now()) {
			w.Header().Set("Retry-After", "60")
			writeProblem(w, http.StatusTooManyRequests, "rate_limited", "organisation rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
