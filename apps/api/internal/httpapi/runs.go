package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runs"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/events"
)

// RunService is the subset of runs.Service the HTTP layer needs.
type RunService interface {
	Start(ctx context.Context, in runs.StartInput) (domain.Run, error)
	Get(ctx context.Context, orgID, id string) (domain.Run, error)
	Events(ctx context.Context, orgID, id string) ([]runs.Event, error)
	EventsAfter(ctx context.Context, orgID, id string, after int64) ([]runs.Event, error)
	Cancel(ctx context.Context, orgID, id string) (domain.Run, error)
	PendingApprovals(ctx context.Context, orgID string, limit int) ([]runs.Approval, error)
	Decide(ctx context.Context, orgID, approvalID, decision, reason, userID string) (domain.Run, string, error)
	ListForTask(ctx context.Context, orgID, taskID string, limit int) ([]domain.Run, error)
	LatestForProject(ctx context.Context, orgID, projectID string) (domain.Run, error)
}

// TaskResolver resolves a task ref to its id.
type TaskResolver interface {
	Get(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
}

type runHandlers struct {
	svc      RunService
	projects ProjectService
	tasks    TaskResolver
	issuer   *auth.Issuer
	log      *slog.Logger
	bus      *events.Bus
	access   ProjectAccessReader
	reauth   time.Duration
}

func newRunHandlers(svc RunService, projects ProjectService, tasks TaskResolver, issuer *auth.Issuer, log *slog.Logger, bus *events.Bus, access ProjectAccessReader, reauth time.Duration) *runHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &runHandlers{svc: svc, projects: projects, tasks: tasks, issuer: issuer, log: log, bus: bus, access: access, reauth: reauth}
}

func (h *runHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/projects/{project}/tasks/{task}/runs", h.start)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/tasks/{task}/runs", h.listForTask)
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}", h.get)
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}/events", h.events)
	mux.HandleFunc("POST /api/v1/orgs/{org}/runs/{run}/cancel", h.cancel)
	mux.HandleFunc("GET /api/v1/orgs/{org}/projects/{project}/mcp-tools", h.mcpTools)
	mux.HandleFunc("GET /api/v1/orgs/{org}/approvals", h.approvals)
	mux.HandleFunc("POST /api/v1/orgs/{org}/approvals/{approval}/decision", h.decide)
}

func (h *runHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
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
	return claims, true
}

func (h *runHandlers) start(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	if _, err := h.projects.Get(r.Context(), claims.OrgID, r.PathValue("project")); err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	var req struct {
		AgentType string `json:"agent_type"`
		Model     string `json:"model"`
		Budget    *struct {
			TokensIn   int   `json:"tokens_in"`
			TokensOut  int   `json:"tokens_out"`
			CostMicros int64 `json:"cost_micros"`
			WallClockS int   `json:"wall_clock_s"`
		} `json:"budget"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if req.AgentType == "" {
		req.AgentType = string(domain.AgentPlanning)
	}

	budget := domain.Budget{}
	if req.Budget != nil {
		budget = domain.Budget{
			TokensIn: req.Budget.TokensIn, TokensOut: req.Budget.TokensOut,
			CostMicros: req.Budget.CostMicros, WallClockS: req.Budget.WallClockS,
		}
	}

	run, err := h.svc.Start(r.Context(), runs.StartInput{
		OrgID: claims.OrgID, ProjectID: r.PathValue("project"),
		TaskID: r.PathValue("task"), AgentType: req.AgentType, Model: req.Model, Budget: budget,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "task not found")
		case errors.Is(err, domain.ErrInvalidTransition):
			writeProblem(w, http.StatusConflict, "invalid_transition", err.Error())
		case errors.Is(err, domain.ErrRateLimited):
			w.Header().Set("Retry-After", "60")
			writeProblem(w, http.StatusTooManyRequests, "rate_limited", "too many active runs for this organisation")
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "conflict", "the task already has an active run")
		default:
			h.log.Error("runs.start", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not start the run")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, runJSON(run))
}

func (h *runHandlers) listForTask(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	project, err := h.projects.Get(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil || project.ID == "" {
		writeProblem(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	task, err := h.tasks.Get(r.Context(), claims.OrgID, project.ID, r.PathValue("task"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "task not found")
		return
	}
	items, err := h.svc.ListForTask(r.Context(), claims.OrgID, task.ID, 20)
	if err != nil {
		h.log.Error("runs.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list runs")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, run := range items {
		data = append(data, runJSON(run))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// canAccessRun enforces project membership for run-scoped routes.
func (h *runHandlers) canAccessRun(ctx context.Context, claims *auth.Claims, projectID string, write bool) bool {
	if claims.Role == "owner" || claims.Role == "admin" || h.access == nil {
		return true
	}
	result, err := h.access.Access(ctx, claims.OrgID, projectID, claims.Subject)
	if err != nil {
		return false
	}
	if result.Members == 0 {
		return true
	}
	if result.Role == "" {
		return false
	}
	return !write || result.Role != "viewer"
}

func (h *runHandlers) get(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	run, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "run not found")
			return
		}
		h.log.Error("runs.get", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the run")
		return
	}
	if !h.canAccessRun(r.Context(), claims, run.ProjectID, false) {
		writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
		return
	}
	writeJSON(w, http.StatusOK, runJSON(run))
}

func (h *runHandlers) events(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	run, err := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if !h.canAccessRun(r.Context(), claims, run.ProjectID, false) {
		writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		h.stream(w, r, claims)
		return
	}
	events, eerr := h.svc.Events(r.Context(), claims.OrgID, r.PathValue("run"))
	if eerr != nil {
		h.log.Error("runs.events", slog.Any("err", eerr))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the events")
		return
	}
	data := make([]map[string]any, 0, len(events))
	for _, e := range events {
		data = append(data, map[string]any{
			"seq": e.Seq, "type": e.Type, "actor": e.Actor,
			"payload": e.Payload, "created_at": e.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// stream pushes events as Server-Sent Events. It polls the database (500ms) so
// it works without Redis; once the API runs multiple replicas this becomes a
// Redis subscription with the same event format.
func (h *runHandlers) stream(w http.ResponseWriter, r *http.Request, claims *auth.Claims) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "internal", "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("retry: 3000\n\n"))
	flusher.Flush()

	runID := r.PathValue("run")
	last := int64(0)
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			last = n
		}
	}

	emit := func(list []runs.Event) bool {
		for _, e := range list {
			data, _ := json.Marshal(map[string]any{"payload": e.Payload, "actor": e.Actor})
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, data); err != nil {
				return false
			}
			last = e.Seq
		}
		if len(list) > 0 {
			flusher.Flush()
		}
		return true
	}

	if initial, err := h.svc.EventsAfter(r.Context(), claims.OrgID, runID, last); err == nil {
		if !emit(initial) {
			return
		}
	}

	// Redis fan-out: a publish only wakes the stream; the rows are still read
	// from the database, so seq/replay semantics do not change. Without Redis
	// the polling loop below is used.
	if h.bus != nil {
		sub, cancel, err := h.bus.Subscribe(r.Context(), events.Channel(runID))
		if err != nil {
			h.log.Warn("runs.stream: subscribe", slog.Any("err", err))
		} else {
			defer cancel()
			heartbeat := time.NewTicker(15 * time.Second)
			defer heartbeat.Stop()
			status := time.NewTicker(5 * time.Second)
			defer status.Stop()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-heartbeat.C:
					_, _ = w.Write([]byte(": ping\n\n"))
					flusher.Flush()
				case <-status.C:
					run, err := h.svc.Get(r.Context(), claims.OrgID, runID)
					if err != nil || run.Status.Terminal() {
						return
					}
				case <-sub:
					list, err := h.svc.EventsAfter(r.Context(), claims.OrgID, runID, last)
					if err != nil {
						return
					}
					if !emit(list) {
						return
					}
				}
			}
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case <-ticker.C:
			events, err := h.svc.EventsAfter(r.Context(), claims.OrgID, runID, last)
			if err != nil {
				return
			}
			for _, e := range events {
				data, _ := json.Marshal(map[string]any{"payload": e.Payload, "actor": e.Actor})
				_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, data)
				last = e.Seq
			}
			if len(events) > 0 {
				flusher.Flush()
				continue
			}

			run, err := h.svc.Get(r.Context(), claims.OrgID, runID)
			if err != nil {
				return
			}
			if run.Status.Terminal() {
				flusher.Flush()
				return
			}
		}
	}
}

func (h *runHandlers) approvals(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.PendingApprovals(r.Context(), claims.OrgID, 50)
	if err != nil {
		h.log.Error("approvals.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list approvals")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, a := range items {
		item := map[string]any{
			"id": a.ID, "run_id": a.RunID, "type": a.Type, "status": a.Status,
			"requested_at": a.RequestedAt.UTC().Format(time.RFC3339),
		}
		if a.PullRequestID != "" {
			item["pull_request_id"] = a.PullRequestID
		}
		if a.Number > 0 {
			item["number"] = a.Number
			item["url"] = a.URL
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *runHandlers) decide(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	if staleApprovalSession(claims, h.reauth, time.Now()) {
		writeProblem(w, http.StatusForbidden, "reauth_required", "please sign in again before deciding an approval")
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if req.Decision == "rejected" && strings.TrimSpace(req.Reason) == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "rejecting a plan requires a reason")
		return
	}
	run, developmentRunID, err := h.svc.Decide(r.Context(), claims.OrgID, r.PathValue("approval"), req.Decision, req.Reason, claims.Subject)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "approval not found")
		case errors.Is(err, domain.ErrInvalidTransition):
			writeProblem(w, http.StatusConflict, "invalid_transition", err.Error())
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("approvals.decide", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not record the decision")
		}
		return
	}
	out := runJSON(run)
	if developmentRunID != "" {
		out["development_run_id"] = developmentRunID
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *runHandlers) cancel(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	current, cerr := h.svc.Get(r.Context(), claims.OrgID, r.PathValue("run"))
	if cerr != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "run not found")
		return
	}
	if !h.canAccessRun(r.Context(), claims, current.ProjectID, true) {
		writeProblem(w, http.StatusForbidden, "forbidden", "you are not a member of this project")
		return
	}
	run, err := h.svc.Cancel(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "not_found", "run not found")
		case errors.Is(err, domain.ErrInvalidTransition):
			writeProblem(w, http.StatusConflict, "invalid_transition", err.Error())
		case errors.Is(err, domain.ErrValidation):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.log.Error("runs.cancel", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not cancel the run")
		}
		return
	}
	writeJSON(w, http.StatusAccepted, runJSON(run))
}

func runJSON(run domain.Run) map[string]any {
	out := map[string]any{
		"id": run.ID, "org_id": run.OrgID, "project_id": run.ProjectID, "task_id": run.TaskID,
		"agent_type": run.AgentType, "status": string(run.Status), "model": run.Model,
		"created_at": run.CreatedAt.UTC().Format(time.RFC3339),
	}
	if run.TraceID != "" {
		out["trace_id"] = run.TraceID
	}
	if len(run.Plan) > 0 {
		out["plan"] = json.RawMessage(run.Plan)
	}
	if len(run.Diff) > 0 {
		out["diff"] = string(run.Diff)
		out["diff_bytes"] = len(run.Diff)
	}
	out["budget"] = map[string]any{
		"tokens_in": run.Budget.TokensIn, "tokens_out": run.Budget.TokensOut,
		"cost_micros": run.Budget.CostMicros,
	}
	if run.CurrentPhase != "" {
		out["current_phase"] = string(run.CurrentPhase)
	}
	if run.FailureCode != "" {
		out["failure_code"] = run.FailureCode
		out["failure_detail"] = run.FailureDetail
	}
	if run.StartedAt != nil {
		out["started_at"] = run.StartedAt.UTC().Format(time.RFC3339)
	}
	if run.FinishedAt != nil {
		out["finished_at"] = run.FinishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// mcpTools returns the MCP tools discovered by the project's newest development
// run (the API does not start sandboxes itself).
func (h *runHandlers) mcpTools(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	run, err := h.svc.LatestForProject(r.Context(), claims.OrgID, r.PathValue("project"))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"tools": []any{}, "run_id": ""})
		return
	}
	events, err := h.svc.Events(r.Context(), claims.OrgID, run.ID)
	if err != nil {
		h.log.Error("runs.mcp_tools", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read the run")
		return
	}
	tools := []map[string]any{}
	for _, event := range events {
		if event.Type != "mcp.tools" {
			continue
		}
		raw, _ := event.Payload["text"].(string)
		var parsed []map[string]any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			tools = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools, "run_id": run.ID})
}

// staleApprovalSession reports whether the approval decision needs a fresh login.
func staleApprovalSession(claims *auth.Claims, window time.Duration, now time.Time) bool {
	if window <= 0 || claims == nil || claims.IssuedAt == nil {
		return false
	}
	return now.Sub(claims.IssuedAt.Time) > window
}
