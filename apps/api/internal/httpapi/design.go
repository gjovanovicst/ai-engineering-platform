package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/design"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/figma"
)

// DesignService manages the token and component registry.
type DesignService interface {
	ListTokens(ctx context.Context, orgID string) ([]design.Token, error)
	CreateToken(ctx context.Context, orgID string, t design.Token) (design.Token, error)
	UpdateToken(ctx context.Context, orgID, id string, t design.Token) (design.Token, error)
	DeleteToken(ctx context.Context, orgID, id string) error
	ListComponents(ctx context.Context, orgID string) ([]design.Component, error)
	CreateComponent(ctx context.Context, orgID string, c design.Component) (design.Component, error)
	UpdateComponent(ctx context.Context, orgID, id string, c design.Component) (design.Component, error)
	DeleteComponent(ctx context.Context, orgID, id string) error
	SeedDefaults(ctx context.Context, orgID string) (int, int, error)
	ImportComponents(ctx context.Context, orgID string, items []design.ImportComponent) (int, int, error)
}

type designHandlers struct {
	figma  *figma.Client
	svc    DesignService
	issuer *auth.Issuer
	log    *slog.Logger
}

func newDesignHandlers(svc DesignService, issuer *auth.Issuer, log *slog.Logger, figmaClient *figma.Client) *designHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &designHandlers{svc: svc, issuer: issuer, log: log, figma: figmaClient}
}

func (h *designHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/design-tokens", h.listTokens)
	mux.HandleFunc("GET /api/v1/orgs/{org}/design-tokens/export", h.exportTokens)
	mux.HandleFunc("POST /api/v1/orgs/{org}/design-tokens", h.createToken)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/design-tokens/{token}", h.updateToken)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/design-tokens/{token}", h.deleteToken)
	mux.HandleFunc("GET /api/v1/orgs/{org}/components", h.listComponents)
	mux.HandleFunc("POST /api/v1/orgs/{org}/components", h.createComponent)
	mux.HandleFunc("POST /api/v1/orgs/{org}/components/import", h.importComponents)
	mux.HandleFunc("POST /api/v1/orgs/{org}/components/figma", h.importFigma)
	mux.HandleFunc("PUT /api/v1/orgs/{org}/components/{component}", h.updateComponent)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/components/{component}", h.deleteComponent)
	mux.HandleFunc("POST /api/v1/orgs/{org}/design/seed", h.seed)
}

func (h *designHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (*auth.Claims, bool) {
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

func (h *designHandlers) exportTokens(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	tokens, err := h.svc.ListTokens(r.Context(), claims.OrgID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read design tokens")
		return
	}
	body, contentType, err := design.Render(r.URL.Query().Get("format"), tokens)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write([]byte(body)) //nolint:gosec // generated CSS/TS, fixed content types
}

func (h *designHandlers) listTokens(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListTokens(r.Context(), claims.OrgID)
	if err != nil {
		h.fail(w, "design.tokens.list", err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, t := range items {
		data = append(data, map[string]any{"id": t.ID, "name": t.Name, "value": t.Value, "category": t.Category})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *designHandlers) createToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	token, err := h.svc.CreateToken(r.Context(), claims.OrgID, design.Token{Name: req.Name, Value: req.Value, Category: req.Category})
	if err != nil {
		h.fail(w, "design.tokens.create", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": token.ID, "name": token.Name, "value": token.Value, "category": token.Category})
}

func (h *designHandlers) updateToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	var req struct {
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	token, err := h.svc.UpdateToken(r.Context(), claims.OrgID, r.PathValue("token"), design.Token{Value: req.Value, Category: req.Category})
	if err != nil {
		h.fail(w, "design.tokens.update", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": token.ID, "name": token.Name, "value": token.Value, "category": token.Category})
}

func (h *designHandlers) deleteToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	if err := h.svc.DeleteToken(r.Context(), claims.OrgID, r.PathValue("token")); err != nil {
		h.fail(w, "design.tokens.delete", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *designHandlers) listComponents(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.ListComponents(r.Context(), claims.OrgID)
	if err != nil {
		h.fail(w, "design.components.list", err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, c := range items {
		data = append(data, componentJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *designHandlers) importFigma(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	if h.figma == nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "AIP_FIGMA_TOKEN is not configured")
		return
	}
	var req struct {
		FileKey string `json:"file_key"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	components, err := h.figma.Components(r.Context(), req.FileKey)
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	items := make([]design.ImportComponent, 0, len(components))
	for _, component := range components {
		items = append(items, design.ImportComponent{
			Name: component.Name, ImportPath: "figma/" + req.FileKey + "/" + component.ID,
			Framework: "figma", Category: "other", Purpose: "Imported from Figma",
		})
	}
	created, skipped, err := h.svc.ImportComponents(r.Context(), claims.OrgID, items)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not import Figma components")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": created, "skipped": skipped})
}

func (h *designHandlers) importComponents(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Components []design.ImportComponent `json:"components"`
	}
	if !decode(w, r, 2<<20, &req) {
		return
	}
	created, skipped, err := h.svc.ImportComponents(r.Context(), claims.OrgID, req.Components)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("design.import_components", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not import components")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": created, "skipped": skipped})
}

func (h *designHandlers) createComponent(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		Purpose    string `json:"purpose"`
		Rules      string `json:"rules"`
		Status     string `json:"status"`
		Category   string `json:"category"`
		Framework  string `json:"framework"`
		Package    string `json:"package"`
		ImportPath string `json:"import_path"`
		Version    string `json:"version"`
		Docs       string `json:"docs"`
		Example    string `json:"example"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	component, err := h.svc.CreateComponent(r.Context(), claims.OrgID, design.Component{
		Name: req.Name, Purpose: req.Purpose, Rules: req.Rules, Status: req.Status,
		Category: req.Category, Framework: req.Framework, Package: req.Package,
		ImportPath: req.ImportPath, Version: req.Version, Docs: req.Docs, Example: req.Example,
	})
	if err != nil {
		h.fail(w, "design.components.create", err)
		return
	}
	writeJSON(w, http.StatusCreated, componentJSON(component))
}

func (h *designHandlers) updateComponent(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		Purpose    string `json:"purpose"`
		Rules      string `json:"rules"`
		Status     string `json:"status"`
		Category   string `json:"category"`
		Framework  string `json:"framework"`
		Package    string `json:"package"`
		ImportPath string `json:"import_path"`
		Version    string `json:"version"`
		Docs       string `json:"docs"`
		Example    string `json:"example"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	component, err := h.svc.UpdateComponent(r.Context(), claims.OrgID, r.PathValue("component"), design.Component{
		Name: req.Name, Purpose: req.Purpose, Rules: req.Rules, Status: req.Status,
		Category: req.Category, Framework: req.Framework, Package: req.Package,
		ImportPath: req.ImportPath, Version: req.Version, Docs: req.Docs, Example: req.Example,
	})
	if err != nil {
		h.fail(w, "design.components.update", err)
		return
	}
	writeJSON(w, http.StatusOK, componentJSON(component))
}

func (h *designHandlers) deleteComponent(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	if err := h.svc.DeleteComponent(r.Context(), claims.OrgID, r.PathValue("component")); err != nil {
		h.fail(w, "design.components.delete", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *designHandlers) seed(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r, "designer")
	if !ok {
		return
	}
	tokens, components, err := h.svc.SeedDefaults(r.Context(), claims.OrgID)
	if err != nil {
		h.fail(w, "design.seed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens, "components": components})
}

func (h *designHandlers) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", "an entry with this name already exists")
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "registry operation failed")
	}
}

func componentJSON(c design.Component) map[string]any {
	return map[string]any{
		"id": c.ID, "name": c.Name, "purpose": c.Purpose, "rules": c.Rules, "status": c.Status,
		"category": c.Category, "framework": c.Framework, "package": c.Package,
		"import_path": c.ImportPath, "version": c.Version, "docs": c.Docs, "example": c.Example,
	}
}
