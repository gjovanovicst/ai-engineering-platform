package httpapi

import (
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// bearerClaims parses the Authorization header and returns the claims.
func bearerClaims(issuer *auth.Issuer, r *http.Request) (*auth.Claims, bool) {
	if claims, ok := claimsFromContext(r.Context()); ok {
		return claims, true
	}
	if issuer == nil {
		return nil, false
	}
	raw, ok := bearerToken(r)
	if !ok {
		return nil, false
	}
	claims, err := issuer.Parse(raw)
	if err != nil {
		return nil, false
	}
	return claims, true
}

// roleAtLeast implements owner > admin > member > viewer.
func roleAtLeast(role, minimum string) bool {
	rank := map[string]int{"viewer": 1, "member": 2, "designer": 3, "admin": 4, "owner": 5}
	r, ok := rank[role]
	if !ok {
		return false
	}
	return r >= rank[minimum]
}
