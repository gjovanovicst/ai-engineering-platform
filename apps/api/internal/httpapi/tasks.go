package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/tasks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/httpapi/oapi"
)

// TaskService is the subset of tasks.Service the HTTP layer needs.
type TaskService interface {
	Create(ctx context.Context, in tasks.CreateInput) (domain.Task, error)
	Get(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
	List(ctx context.Context, orgID, projectID, status, assignee string, limit int) ([]domain.Task, error)
	Transition(ctx context.Context, orgID, idOrRef, projectID, to, reason string) (domain.Task, error)
	MarkMerged(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
	Update(ctx context.Context, in tasks.UpdateInput) (domain.Task, error)
}

type taskHandlers struct {
	svc      TaskService
	projects ProjectService
	issuer   *auth.Issuer
	log      *slog.Logger
}

func newTaskHandlers(svc TaskService, projects ProjectService, issuer *auth.Issuer, log *slog.Logger) *taskHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &taskHandlers{svc: svc, projects: projects, issuer: issuer, log: log}
}

func (h *taskHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/tasks", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/tasks", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/tasks/{task}", h.get)
	mux.HandleFunc("PATCH /api/v1/orgs/{org}/projects/{project}/tasks/{task}", h.update)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/tasks/{task}/transitions", h.transition)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/scaffold/component", h.scaffoldComponent)
}

func (h *taskHandlers) scaffoldComponent(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	project, err := h.projects.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	spec, err := componentScaffold(req.Name, req.Kind, project.DesignStandardID)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	task, err := h.svc.Create(r.Context(), tasks.CreateInput{
		OrgID: claims.OrgID, ProjectID: project.ID, Title: spec.Title, Description: spec.Description,
		Acceptance: spec.Acceptance, Priority: domain.PriorityMed, Tags: spec.Tags, CreatedBy: claims.Subject,
	})
	if err != nil {
		h.log.Error("tasks.scaffold", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not create the scaffold task")
		return
	}
	writeJSON(w, http.StatusCreated, taskJSON(task))
}

func (h *taskHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return nil, false
	}
	if org := r.PathValue("org"); org != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	// The project must belong to the caller's org; a foreign id is a 404.
	if _, err := h.projects.Get(r.Context(), claims.OrgID, r.PathValue("project")); err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return nil, false
	}
	return claims, true
}

type acceptanceJSON struct {
	Given string `json:"given"`
	When  string `json:"when"`
	Then  string `json:"then"`
}

func (h *taskHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req oapi.TaskInput
	if !decode(w, r, 1<<20, &req) {
		return
	}
	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	priority := domain.Priority("")
	if req.Priority != nil {
		priority = domain.Priority(*req.Priority)
	}
	var acceptance []domain.AcceptanceCriterion
	if req.Acceptance != nil {
		for _, a := range *req.Acceptance {
			acceptance = append(acceptance, domain.AcceptanceCriterion{Given: a.Given, When: a.When, Then: a.Then})
		}
	}
	var tags []string
	if req.Tags != nil {
		tags = *req.Tags
	}
	var dependsOn []string
	if req.DependsOn != nil {
		dependsOn = *req.DependsOn
	}
	var files, outOfScope, testPlan, rollback []string
	if req.Files != nil {
		files = *req.Files
	}
	if req.OutOfScope != nil {
		outOfScope = *req.OutOfScope
	}
	if req.TestPlan != nil {
		testPlan = *req.TestPlan
	}
	if req.Rollback != nil {
		rollback = *req.Rollback
	}

	task, err := h.svc.Create(r.Context(), tasks.CreateInput{
		OrgID: claims.OrgID, ProjectID: r.PathValue("project"),
		Title: req.Title, Description: description, DependsOn: dependsOn,
		Acceptance: acceptance, Priority: priority, Tags: tags,
		CreatedBy: claims.Subject,
		Files:     files, OutOfScope: outOfScope, TestPlan: testPlan, Rollback: rollback,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "could not allocate a task reference")
		default:
			h.log.Error("tasks.create", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not create the task")
		}
		return
	}
	writeJSON(w, http.StatusCreated, taskJSON(task))
}

func (h *taskHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "limit must be a positive integer")
			return
		}
		limit = n
	}

	items, err := h.svc.List(r.Context(), claims.OrgID, r.PathValue("project"),
		r.URL.Query().Get("status"), r.URL.Query().Get("assignee"), limit)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("tasks.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list tasks")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, t := range items {
		data = append(data, taskJSON(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": nil})
}

func (h *taskHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	task, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("project"), r.PathValue("task"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "task not found")
			return
		}
		h.log.Error("tasks.get", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the task")
		return
	}
	writeJSON(w, http.StatusOK, taskJSON(task))
}

func (h *taskHandlers) transition(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		To     string `json:"to"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}

	task, err := h.svc.Transition(r.Context(), claims.OrgID, r.PathValue("task"),
		r.PathValue("project"), req.To, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "task not found")
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrInvalidTransition):
			writeProblem(w, http.StatusConflict, "invalid_transition", err.Error())
		default:
			h.log.Error("tasks.transition", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not move the task")
		}
		return
	}
	writeJSON(w, http.StatusOK, taskJSON(task))
}

func taskJSON(t domain.Task) map[string]any {
	acceptance := make([]acceptanceJSON, 0, len(t.Acceptance))
	for _, a := range t.Acceptance {
		acceptance = append(acceptance, acceptanceJSON{Given: a.Given, When: a.When, Then: a.Then})
	}
	out := map[string]any{
		"id":           t.ID,
		"org_id":       t.OrgID,
		"project_id":   t.ProjectID,
		"ref":          t.Ref,
		"title":        t.Title,
		"description":  t.Description,
		"acceptance":   acceptance,
		"status":       string(t.Status),
		"priority":     string(t.Priority),
		"tags":         t.Tags,
		"depends_on":   t.DependsOn,
		"files":        t.Files,
		"out_of_scope": t.OutOfScope,
		"test_plan":    t.TestPlan,
		"rollback":     t.Rollback,
		"created_at":   t.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":   t.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if t.BlockedReason != "" {
		out["blocked_reason"] = t.BlockedReason
	}
	return out
}

func (h *taskHandlers) update(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Priority    domain.Priority `json:"priority"`
		AssigneeID  string          `json:"assignee_id"`
		Tags        []string        `json:"tags"`
		Acceptance  *[]struct {
			Given string `json:"given"`
			When  string `json:"when"`
			Then  string `json:"then"`
		} `json:"acceptance"`
		DependsOn  *[]string `json:"depends_on"`
		Files      *[]string `json:"files"`
		OutOfScope *[]string `json:"out_of_scope"`
		TestPlan   *[]string `json:"test_plan"`
		Rollback   *[]string `json:"rollback"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	var acceptance []domain.AcceptanceCriterion
	if req.Acceptance != nil {
		for _, a := range *req.Acceptance {
			acceptance = append(acceptance, domain.AcceptanceCriterion{Given: a.Given, When: a.When, Then: a.Then})
		}
	}
	var dependsOn []string
	if req.DependsOn != nil {
		dependsOn = *req.DependsOn
	}
	var files, outOfScope, testPlan, rollback []string
	if req.Files != nil {
		files = *req.Files
	}
	if req.OutOfScope != nil {
		outOfScope = *req.OutOfScope
	}
	if req.TestPlan != nil {
		testPlan = *req.TestPlan
	}
	if req.Rollback != nil {
		rollback = *req.Rollback
	}
	task, err := h.svc.Update(r.Context(), tasks.UpdateInput{
		OrgID: claims.OrgID, ProjectID: r.PathValue("project"), IDOrRef: r.PathValue("task"),
		Title: req.Title, Description: req.Description, Priority: req.Priority,
		AssigneeID: req.AssigneeID, Tags: req.Tags,
		Acceptance: acceptance, DependsOn: dependsOn,
		Files: files, OutOfScope: outOfScope, TestPlan: testPlan, Rollback: rollback,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "task not found")
		case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrInvalidTransition):
			writeProblem(w, http.StatusConflict, "conflict", err.Error())
		default:
			writeProblem(w, http.StatusInternalServerError, "internal", "could not update the task")
		}
		return
	}
	writeJSON(w, http.StatusOK, taskJSON(task))
}
