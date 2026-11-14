package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// OrgRegionReader resolves the org's data-plane region.
type OrgRegionReader interface {
	Region(ctx context.Context, orgID string) (string, error)
}

// withRegionGuard rejects org requests that belong to another data plane.
// It is opt-in: an empty AIP_REGION disables it.
func withRegionGuard(next http.Handler, issuer *auth.Issuer, region string, regions OrgRegionReader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/api/v1/orgs/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			next.ServeHTTP(w, r)
			return
		}
		orgID := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")[0]
		if orgID == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, ok := bearerClaims(issuer, r)
		if !ok || claims.OrgID != orgID {
			next.ServeHTTP(w, r) // the handler returns the proper 401/404
			return
		}
		actual, err := regions.Region(r.Context(), orgID)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if actual != "" && actual != region {
			writeProblem(w, http.StatusMisdirectedRequest, "wrong_region", "this organisation belongs to another data plane")
			return
		}
		next.ServeHTTP(w, r)
	})
}
