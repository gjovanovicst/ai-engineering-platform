package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/agentplugins"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// AgentPluginService registers and selects custom agents.
type AgentPluginService interface {
	Create(ctx context.Context, orgID, actorID, name, version, image, command string, types []string) (agentplugins.Plugin, error)
	List(ctx context.Context, orgID string) ([]agentplugins.Plugin, error)
	Select(ctx context.Context, orgID, projectID, pluginID, actorID string) error
}

type agentPluginHandlers struct {
	svc    AgentPluginService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *agentPluginHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/agent-plugins", h.list)
	mux.HandleFunc("POST /api/v1/orgs/{org}/agent-plugins", h.create)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/projects/{project}/agent-plugin", h.selectPlugin)
}

func (h *agentPluginHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return nil, false
	}
	return claims, true
}

func agentPluginJSON(p agentplugins.Plugin) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "version": p.Version, "image": p.Image,
		"command": p.Command, "agent_types": p.AgentTypes,
		"created_at": p.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *agentPluginHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	plugins, err := h.svc.List(r.Context(), claims.OrgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list agent plugins")
		return
	}
	out := make([]map[string]any, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, agentPluginJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *agentPluginHandlers) create(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		Name       string   `json:"name"`
		Version    string   `json:"version"`
		Image      string   `json:"image"`
		Command    string   `json:"command"`
		AgentTypes []string `json:"agent_types"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	plugin, err := h.svc.Create(r.Context(), claims.OrgID, claims.Subject, req.Name, req.Version, req.Image, req.Command, req.AgentTypes)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			writeProblem(w, http.StatusConflict, "conflict", "this plugin version already exists")
			return
		}
		h.log.Error("agent_plugins.create", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not save the plugin")
		return
	}
	writeJSON(w, http.StatusCreated, agentPluginJSON(plugin))
}

func (h *agentPluginHandlers) selectPlugin(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var req struct {
		PluginID string `json:"plugin_id"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if err := h.svc.Select(r.Context(), claims.OrgID, r.PathValue("project"), req.PluginID, claims.Subject); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "project or plugin not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not select the plugin")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
