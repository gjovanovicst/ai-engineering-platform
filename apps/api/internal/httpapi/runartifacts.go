package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/runartifacts"
)

// ArtifactLister lists a run's stored artifacts.
type ArtifactLister interface {
	ListForRun(ctx context.Context, orgID, runID string) ([]runartifacts.Artifact, error)
}

type artifactHandlers struct {
	svc     ArtifactLister
	issuer  *auth.Issuer
	objects ObjectStore
	log     *slog.Logger
}

func newArtifactHandlers(svc ArtifactLister, issuer *auth.Issuer, objects ObjectStore, log *slog.Logger) *artifactHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &artifactHandlers{svc: svc, issuer: issuer, objects: objects, log: log}
}

func (h *artifactHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}/artifacts", h.list)
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs/{run}/visual", h.visual)
}

func (h *artifactHandlers) visual(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" || strings.ContainsAny(name, "/\\") {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	if h.objects == nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "S3 is not configured")
		return
	}
	key := fmt.Sprintf("runs/%s/%s/visual/%s", claims.OrgID, r.PathValue("run"), name)
	data, err := h.objects.Get(r.Context(), key)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "visual artifact not found")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = w.Write(data) //nolint:gosec // PNG bytes, fixed content type
}

func (h *artifactHandlers) list(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	items, err := h.svc.ListForRun(r.Context(), claims.OrgID, r.PathValue("run"))
	if err != nil {
		h.log.Error("artifacts.list", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list artifacts")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, map[string]any{
			"kind": item.Kind, "key": item.BucketKey, "bytes": item.Bytes,
			"sha256":     fmt.Sprintf("%x", item.SHA256),
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
