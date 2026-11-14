package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/directory"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// DirectoryService lists workspace-wide resources.
type DirectoryService interface {
	Repos(ctx context.Context, orgID string, limit int) ([]directory.Repo, error)
	Runs(ctx context.Context, orgID, status, agentType string, limit int) ([]directory.Run, error)
	Members(ctx context.Context, orgID string, limit int) ([]directory.Member, error)
	Audit(ctx context.Context, orgID string, limit int) ([]directory.AuditEntry, error)
	UpdateRole(ctx context.Context, orgID, actorID, userID, role string) error
	RemoveMember(ctx context.Context, orgID, actorID, userID string) error
}

type directoryHandlers struct {
	svc             DirectoryService
	issuer          *auth.Issuer
	log             *slog.Logger
	auditSigningKey []byte
	auditEd25519    ed25519.PrivateKey
}

func newDirectoryHandlers(svc DirectoryService, issuer *auth.Issuer, log *slog.Logger, auditSigningKey string, auditEd25519 ed25519.PrivateKey) *directoryHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &directoryHandlers{svc: svc, issuer: issuer, log: log, auditSigningKey: []byte(auditSigningKey), auditEd25519: auditEd25519}
}

func (h *directoryHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/orgs/{org}/repos", h.repos)
	mux.HandleFunc("GET /api/v1/orgs/{org}/runs", h.runs)
	mux.HandleFunc("GET /api/v1/orgs/{org}/members", h.members)
	mux.HandleFunc("GET /api/v1/orgs/{org}/audit", h.audit)
	mux.HandleFunc("GET /api/v1/orgs/{org}/audit/export", h.auditExport)
	mux.HandleFunc("GET /api/v1/audit/public-key", h.auditPublicKey)
	mux.HandleFunc("PATCH /api/v1/orgs/{org}/members/{user}", h.updateMemberRole)
	mux.HandleFunc("DELETE /api/v1/orgs/{org}/members/{user}", h.removeMember)
}

func (h *directoryHandlers) authorize(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return nil, false
	}
	return claims, true
}

func (h *directoryHandlers) repos(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Repos(r.Context(), claims.OrgID, 100)
	if err != nil {
		h.log.Error("directory.repos", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list repositories")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, repo := range items {
		data = append(data, map[string]any{
			"id": repo.ID, "project_id": repo.ProjectID, "project_name": repo.ProjectName,
			"provider": repo.Provider, "name": repo.Name, "default_branch": repo.DefaultBranch,
			"created_at": repo.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *directoryHandlers) runs(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Runs(r.Context(), claims.OrgID, r.URL.Query().Get("status"), r.URL.Query().Get("agent_type"), 100)
	if err != nil {
		h.log.Error("directory.runs", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list runs")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, run := range items {
		item := map[string]any{
			"id": run.ID, "project_id": run.ProjectID, "task_ref": run.TaskRef,
			"task_title": run.TaskTitle, "agent_type": run.AgentType, "status": run.Status,
			"created_at": run.CreatedAt.UTC().Format(time.RFC3339),
		}
		if run.FinishedAt != nil {
			item["finished_at"] = run.FinishedAt.UTC().Format(time.RFC3339)
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *directoryHandlers) members(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Members(r.Context(), claims.OrgID, 100)
	if err != nil {
		h.log.Error("directory.members", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not list members")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, member := range items {
		data = append(data, map[string]any{
			"user_id": member.UserID, "email": member.Email, "name": member.Name,
			"role": member.Role, "joined_at": member.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *directoryHandlers) audit(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Audit(r.Context(), claims.OrgID, 100)
	if err != nil {
		h.log.Error("directory.audit", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read the audit log")
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, entry := range items {
		item := map[string]any{
			"id": entry.ID, "actor_kind": entry.ActorKind, "action": entry.Action,
			"target": entry.Target, "created_at": entry.CreatedAt.UTC().Format(time.RFC3339),
			"metadata": json.RawMessage(entry.Metadata), "prev_hash": entry.PrevHash, "hash": entry.Hash,
		}
		if entry.ActorID != "" {
			item["actor_id"] = entry.ActorID
		}
		data = append(data, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *directoryHandlers) auditPublicKey(w http.ResponseWriter, r *http.Request) {
	if h.auditEd25519 == nil {
		writeProblem(w, http.StatusNotFound, "not_found", "no asymmetric audit key is configured")
		return
	}
	pub := h.auditEd25519.Public().(ed25519.PublicKey)
	writeJSON(w, http.StatusOK, map[string]any{"alg": "ed25519", "public_key": base64.StdEncoding.EncodeToString(pub)})
}

func (h *directoryHandlers) auditExport(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	items, err := h.svc.Audit(r.Context(), claims.OrgID, 10000)
	if err != nil {
		h.log.Error("directory.audit_export", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not export the audit log")
		return
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, entry := range items {
		_ = enc.Encode(map[string]any{
			"id": entry.ID, "actor_id": entry.ActorID, "actor_kind": entry.ActorKind,
			"action": entry.Action, "target": entry.Target, "metadata": json.RawMessage(entry.Metadata),
			"created_at": entry.CreatedAt.UTC().Format(time.RFC3339), "prev_hash": entry.PrevHash, "hash": entry.Hash,
		})
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="audit.ndjson"`)
	if h.auditEd25519 != nil {
		signature := ed25519.Sign(h.auditEd25519, buf.Bytes())
		w.Header().Set("X-AIP-Audit-Signature", "ed25519="+base64.StdEncoding.EncodeToString(signature))
	} else if signature := auditSignature(h.auditSigningKey, buf.Bytes()); signature != "" {
		w.Header().Set("X-AIP-Audit-Signature", "sha256="+signature)
	}
	_, _ = w.Write(buf.Bytes())
}

// auditSignature returns an HMAC-SHA256 hex digest, or "" when no key is set.
func auditSignature(key, body []byte) string {
	if len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *directoryHandlers) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	if err := h.svc.UpdateRole(r.Context(), claims.OrgID, claims.Subject, r.PathValue("user"), req.Role); err != nil {
		h.memberError(w, "directory.member_role", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *directoryHandlers) removeMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return
	}
	if err := h.svc.RemoveMember(r.Context(), claims.OrgID, claims.Subject, r.PathValue("user")); err != nil {
		h.memberError(w, "directory.member_remove", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *directoryHandlers) memberError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "member not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "member operation failed")
	}
}
