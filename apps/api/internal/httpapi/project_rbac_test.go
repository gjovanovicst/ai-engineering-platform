package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projectaccess"
)

type fakeAccess struct{ result projectaccess.Access }

func (f fakeAccess) Access(context.Context, string, string, string) (projectaccess.Access, error) {
	return f.result, nil
}
func (f fakeAccess) SetRole(context.Context, string, string, string, string) (projectaccess.Member, error) {
	return projectaccess.Member{}, nil
}
func (f fakeAccess) List(context.Context, string, string) ([]projectaccess.Member, error) {
	return nil, nil
}
func (f fakeAccess) Delete(context.Context, string, string, string) error { return nil }

func TestProjectRBAC(t *testing.T) {
	const org = "11111111-1111-1111-1111-111111111111"
	const project = "22222222-2222-2222-2222-222222222222"
	issuer := testIssuer(t)
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	cases := []struct {
		name   string
		role   string
		method string
		access projectaccess.Access
		want   int
	}{
		{"no members", "member", "POST", projectaccess.Access{}, 200},
		{"viewer write", "member", "POST", projectaccess.Access{Role: "viewer", Members: 1}, 403},
		{"viewer read", "member", "GET", projectaccess.Access{Role: "viewer", Members: 1}, 200},
		{"outsider", "member", "GET", projectaccess.Access{Members: 1}, 403},
		{"org admin", "admin", "POST", projectaccess.Access{Members: 1}, 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := withProjectRBAC(okHandler, issuer, fakeAccess{result: tc.access})
			req := httptest.NewRequest(tc.method, "/api/v1/orgs/"+org+"/projects/"+project+"/tasks", nil)
			req.Header.Set("Authorization", "Bearer "+tokenFor(t, issuer, org, tc.role))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("code = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestProjectVisible(t *testing.T) {
	h := &projectHandlers{access: fakeAccess{result: projectaccess.Access{Members: 1}}}
	claims := &auth.Claims{OrgID: "o", Role: "member"}
	if h.projectVisible(context.Background(), claims, "p") {
		t.Fatal("outsider project must be hidden")
	}
	h.access = fakeAccess{result: projectaccess.Access{Members: 1, Role: "viewer"}}
	if !h.projectVisible(context.Background(), claims, "p") {
		t.Fatal("member project must be visible")
	}
	claims.Role = "admin"
	h.access = fakeAccess{result: projectaccess.Access{Members: 1}}
	if !h.projectVisible(context.Background(), claims, "p") {
		t.Fatal("org admin must see all projects")
	}
}
