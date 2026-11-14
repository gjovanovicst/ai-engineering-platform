package httpapi

import (
	"context"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projectaccess"
)

func TestCanAccessRun(t *testing.T) {
	h := &runHandlers{access: fakeAccess{result: projectaccess.Access{Members: 1}}}
	claims := &auth.Claims{OrgID: "o", Role: "member"}
	claims.Subject = "u"
	if h.canAccessRun(context.Background(), claims, "p", false) {
		t.Fatal("outsider must be denied")
	}
	claims.Role = "admin"
	if !h.canAccessRun(context.Background(), claims, "p", true) {
		t.Fatal("org admin must pass")
	}
	h.access = fakeAccess{result: projectaccess.Access{Role: "viewer", Members: 1}}
	claims.Role = "member"
	if h.canAccessRun(context.Background(), claims, "p", true) {
		t.Fatal("viewer must not write")
	}
	if !h.canAccessRun(context.Background(), claims, "p", false) {
		t.Fatal("viewer must read")
	}
}
