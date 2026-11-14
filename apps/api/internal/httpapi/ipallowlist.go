package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// withIPAllowlist rejects API clients outside the configured CIDRs. Health,
// metrics and provider webhooks stay reachable.
func withIPAllowlist(next http.Handler, allowed []netip.Prefix) http.Handler {
	if len(allowed) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz", r.URL.Path == "/metrics", strings.HasPrefix(r.URL.Path, "/webhooks/"):
			next.ServeHTTP(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			writeProblem(w, http.StatusForbidden, "forbidden", "client address is not allowed")
			return
		}
		addr = addr.Unmap()
		for _, prefix := range allowed {
			if prefix.Contains(addr) {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeProblem(w, http.StatusForbidden, "forbidden", "client address is not allowed")
	})
}
