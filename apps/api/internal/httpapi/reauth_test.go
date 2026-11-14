package httpapi

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

func TestStaleApprovalSession(t *testing.T) {
	now := time.Now()
	claims := &auth.Claims{RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now.Add(-20 * time.Minute))}}
	if !staleApprovalSession(claims, 15*time.Minute, now) {
		t.Fatal("20m old token must require re-auth with a 15m window")
	}
	if staleApprovalSession(claims, 0, now) {
		t.Fatal("a zero window disables the check")
	}
	claims.IssuedAt = jwt.NewNumericDate(now.Add(-5 * time.Minute))
	if staleApprovalSession(claims, 15*time.Minute, now) {
		t.Fatal("5m old token must pass")
	}
}
