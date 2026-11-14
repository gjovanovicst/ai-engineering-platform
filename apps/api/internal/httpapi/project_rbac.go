package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/projectaccess"
)

// ProjectAccessReader checks per-project membership.
type ProjectAccessReader interface {
	Access(ctx context.Context, orgID, projectID, userID string) (projectaccess.Access, error)
}

// withProjectRBAC enforces project membership once a project has explicit
// members. Org owners/admins bypass it; projects without members stay org-wide.
func withProjectRBAC(next http.Handler, issuer *auth.Issuer, access ProjectAccessReader) http.Handler {
	if access == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID, projectID, ok := projectPath(r.URL.Path)
		if !ok || !isUUID(projectID) {
			next.ServeHTTP(w, r)
			return
		}
		claims, valid := bearerClaims(issuer, r)
		if !valid || claims.OrgID != orgID {
			next.ServeHTTP(w, r)
			return
		}
		if claims.Role == "owner" || claims.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}
		result, err := access.Access(r.Context(), orgID, projectID, claims.Subject)
		if err != nil || result.Members == 0 {
			next.ServeHTTP(w, r)
			return
		}
		if result.Role == "" {
			writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
			return
		}
		if result.Role == "viewer" && r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeProblem(w, http.StatusForbidden, "forbidden", "project viewers cannot modify resources")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func projectPath(path string) (orgID, projectID string, ok bool) {
	const prefix = "/api/v1/orgs/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 3 || parts[1] != "projects" {
		return "", "", false
	}
	return parts[0], parts[2], true
}

// ProjectAccessService is the member management use case.
type ProjectAccessService interface {
	ProjectAccessReader
	SetRole(ctx context.Context, orgID, projectID, userID, role string) (projectaccess.Member, error)
	List(ctx context.Context, orgID, projectID string) ([]projectaccess.Member, error)
	Delete(ctx context.Context, orgID, projectID, userID string) error
}

type projectMemberHandlers struct {
	svc    ProjectAccessService
	issuer *auth.Issuer
}

func (h *projectMemberHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/members", h.list)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/members/{user}", h.set)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/projects/{project}/members/{user}", h.remove)
}

func (h *projectMemberHandlers) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return "", false
	}
	return claims.OrgID, true
}

func (h *projectMemberHandlers) list(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	members, err := h.svc.List(r.Context(), orgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read project members")
		return
	}
	out := make([]map[string]any, 0, len(members))
	for _, member := range members {
		out = append(out, map[string]any{"user_id": member.UserID, "role": member.Role, "created_at": member.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *projectMemberHandlers) set(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	member, err := h.svc.SetRole(r.Context(), orgID, r.PathValue("project"), r.PathValue("user"), req.Role)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": member.UserID, "role": member.Role})
}

func (h *projectMemberHandlers) remove(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, r.PathValue("project"), r.PathValue("user")); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not remove the project member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
