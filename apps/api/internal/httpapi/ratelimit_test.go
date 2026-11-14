package httpapi

import (
	"testing"
	"time"
)

func TestOrgRateLimiterWindow(t *testing.T) {
	limiter := newOrgRateLimiter(2)
	now := time.Now()
	first := limiter.allow("org-1", now)
	second := limiter.allow("org-1", now)
	if !first || !second {
		t.Fatal("first two requests must pass")
	}
	if limiter.allow("org-1", now) {
		t.Fatal("third request in the window must be limited")
	}
	if !limiter.allow("org-2", now) {
		t.Fatal("another org has its own window")
	}
	if !limiter.allow("org-1", now.Add(time.Minute)) {
		t.Fatal("a new window must reset the count")
	}
}
