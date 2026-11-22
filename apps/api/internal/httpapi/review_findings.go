package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// ReviewFindingService is the subset of the findings store the HTTP layer needs.
type ReviewFindingService interface {
	List(ctx context.Context, orgID, taskID string) ([]domain.ReviewFinding, error)
	Decide(ctx context.Context, orgID, id, status, by, reason string) (domain.ReviewFinding, error)
}

// reviewFindingHandlers serves the findings of a task and their decisions.
type reviewFindingHandlers struct {
	svc     ReviewFindingService
	project *runHandlers
}

func newReviewFindingHandlers(svc ReviewFindingService, project *runHandlers) *reviewFindingHandlers {
	return &reviewFindingHandlers{svc: svc, project: project}
}

func (h *reviewFindingHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/tasks/{task}/review-findings", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/tasks/{task}/review-findings/{finding}/decide", h.decide)
}

func reviewFindingJSON(f domain.ReviewFinding) map[string]any {
	return map[string]any{
		"id": f.ID, "task_id": f.TaskID, "run_id": f.RunID, "severity": string(f.Severity),
		"file": f.File, "line": f.Line, "message": f.Message, "status": string(f.Status),
		"decided_by": f.DecidedBy, "reason": f.Reason, "fixed_by_run": f.FixedByRun,
		"created_at": f.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *reviewFindingHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.project.authorize(w, r, "member")
	if !ok {
		return
	}
	if !h.project.canAccessRun(r.Context(), claims, r.PathValue("project"), true) {
		writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
		return
	}
	findings, err := h.svc.List(r.Context(), claims.OrgID, r.PathValue("task"))
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list the review findings")
		return
	}
	out := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		out = append(out, reviewFindingJSON(finding))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *reviewFindingHandlers) decide(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.project.authorize(w, r, "member")
	if !ok {
		return
	}
	if !h.project.canAccessRun(r.Context(), claims, r.PathValue("project"), true) {
		writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	status := ""
	switch req.Decision {
	case "dismiss":
		status = string(domain.ReviewFindingDismissed)
		if strings.TrimSpace(req.Reason) == "" {
			writeProblem(w, http.StatusBadRequest, "validation_failed", "a dismissal needs a reason")
			return
		}
	case "reopen":
		status = string(domain.ReviewFindingOpen)
	default:
		writeProblem(w, http.StatusBadRequest, "validation_failed", "decision must be dismiss or reopen")
		return
	}
	finding, err := h.svc.Decide(r.Context(), claims.OrgID, r.PathValue("finding"), status, claims.Subject, strings.TrimSpace(req.Reason))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "finding not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not decide the finding")
		return
	}
	writeJSON(w, http.StatusOK, reviewFindingJSON(finding))
}
