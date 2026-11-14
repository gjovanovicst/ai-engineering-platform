package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/rules"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RulesService manages versioned rules.
type RulesService interface {
	Create(ctx context.Context, in rules.CreateInput) (rules.Rule, rules.Version, error)
	List(ctx context.Context, orgID, projectID string, limit int) ([]rules.Rule, error)
	Versions(ctx context.Context, orgID, ruleID string) ([]rules.Version, error)
	SaveDraft(ctx context.Context, orgID, ruleID, body string) (rules.Version, error)
	Publish(ctx context.Context, orgID, ruleID string, version int, userID string) (rules.Version, error)
	RequestPublish(ctx context.Context, orgID, ruleID string, version int, userID string) (rules.PublishRequest, error)
	PendingPublishes(ctx context.Context, orgID string) ([]rules.PublishRequest, error)
	DecidePublish(ctx context.Context, orgID, approvalID, decision, userID, reason string) (rules.Version, error)
}

type ruleHandlers struct {
	svc    RulesService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newRuleHandlers(svc RulesService, issuer *auth.Issuer, log *slog.Logger) *ruleHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &ruleHandlers{svc: svc, issuer: issuer, log: log}
}

func (h *ruleHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/rules", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/rules", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/rules/{rule}/versions", h.versions)
	mux.HandleFunc("POST /api/v1/orgs/{org}/rules/{rule}/versions", h.saveDraft)
	mux.HandleFunc("POST /api/v1/orgs/{org}/rules/{rule}/versions/{version}/publish", h.publish)
	mux.HandleFunc("GET /api/v1/orgs/{org}/rules/approvals", h.pendingPublishes)
	mux.HandleFunc("POST /api/v1/orgs/{org}/rules/approvals/{approval}/decide", h.decidePublish)
}

func (h *ruleHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return nil, false
	}
	return claims, true
}

func (h *ruleHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), claims.OrgID, r.URL.Query().Get("project"), 100)
	if err != nil {
		h.log.Error("rules.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list rules")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, rule := range items {
		item := map[string]any{
			"id": rule.ID, "name": rule.Name, "level": rule.Level,
			"created_at": rule.CreatedAt.UTC().Format(time.RFC3339),
		}
		if rule.ProjectID != "" {
			item["project_id"] = rule.ProjectID
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *ruleHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Name      string `json:"name"`
		Level     string `json:"level"`
		ProjectID string `json:"project_id"`
		ScopeRef  string `json:"scope_ref"`
		Body      string `json:"body"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	rule, version, err := h.svc.Create(r.Context(), rules.CreateInput{
		OrgID: claims.OrgID, ProjectID: req.ProjectID, Level: req.Level,
		ScopeRef: req.ScopeRef, Name: req.Name, Body: req.Body,
	})
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": rule.ID, "name": rule.Name, "level": rule.Level,
		"version": version.Version, "published": false,
	})
}

func (h *ruleHandlers) versions(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.Versions(r.Context(), claims.OrgID, r.PathValue("rule"))
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, version := range items {
		item := map[string]any{
			"version": version.Version, "body": version.Body,
			"published":  version.PublishedAt != nil,
			"created_at": version.CreatedAt.UTC().Format(time.RFC3339),
		}
		if version.PublishedAt != nil {
			item["published_at"] = version.PublishedAt.UTC().Format(time.RFC3339)
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *ruleHandlers) saveDraft(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	version, err := h.svc.SaveDraft(r.Context(), claims.OrgID, r.PathValue("rule"), req.Body)
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": version.Version, "published": false})
}

func (h *ruleHandlers) publish(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	version, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "invalid version")
		return
	}
	request, err := h.svc.RequestPublish(r.Context(), claims.OrgID, r.PathValue("rule"), version, claims.Subject)
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"approval_id": request.ID, "status": "pending", "version": request.Version,
	})
}

func (h *ruleHandlers) writeRuleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "rule not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", "a rule with this name already exists")
	case errors.Is(err, domain.ErrInvalidTransition):
		writeProblem(w, http.StatusConflict, "invalid_transition", "the version is already published")
	default:
		h.log.Error("rules", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "rules operation failed")
	}
}

func (h *ruleHandlers) pendingPublishes(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.PendingPublishes(r.Context(), claims.OrgID)
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{
			"id": item.ID, "rule_id": item.RuleID, "rule": item.RuleName, "version": item.Version,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *ruleHandlers) decidePublish(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	published, err := h.svc.DecidePublish(r.Context(), claims.OrgID, r.PathValue("approval"), req.Decision, claims.Subject, req.Reason)
	if err != nil {
		h.writeRuleError(w, err)
		return
	}
	if req.Decision == "approved" {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": published.Version, "published": true,
			"published_at": published.PublishedAt.UTC().Format(time.RFC3339),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"published": false, "status": "rejected"})
}
