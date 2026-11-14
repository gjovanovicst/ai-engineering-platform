package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/attachments"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// AttachmentService is the task-attachment use case.
type AttachmentService interface {
	Upload(ctx context.Context, orgID, projectID, taskID, userID, filename, contentType string, data []byte) (attachments.Meta, error)
	List(ctx context.Context, orgID, taskID string) ([]attachments.Meta, error)
	Get(ctx context.Context, orgID, taskID, id string) (attachments.Attachment, error)
}

type attachmentHandlers struct {
	svc    AttachmentService
	issuer *authIssuer
	log    *slog.Logger
}

// newAttachmentHandlers is declared in attachments_wire.go to keep auth imports
// close to the other handlers.

func (h *attachmentHandlers) registerRoutes(mux *http.ServeMux) {
	base := "/api/v1/orgs/{org}/projects/{project}/tasks/{task}/attachments"
	mux.HandleFunc("GET "+base, h.list)
	mux.HandleFunc("POST "+base, h.upload)
	mux.HandleFunc("GET "+base+"/{attachment}", h.download)
}

func (h *attachmentHandlers) authorize(w http.ResponseWriter, r *http.Request, minRole string) (orgID, userID string, ok bool) {
	claims, valid := bearerClaims(h.issuer, r)
	if !valid || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", "", false
	}
	if !roleAtLeast(claims.Role, minRole) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return "", "", false
	}
	return claims.OrgID, claims.Subject, true
}

func (h *attachmentHandlers) list(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), orgID, r.PathValue("task"))
	if err != nil {
		h.fail(w, "attachments.list", err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, attachmentJSON(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *attachmentHandlers) upload(w http.ResponseWriter, r *http.Request) {
	orgID, userID, ok := h.authorize(w, r, "member")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, attachments.MaxBytes+1<<20)
	if err := r.ParseMultipartForm(attachments.MaxBytes + 1<<20); err != nil { //nolint:gosec // body is capped by MaxBytesReader above
		writeProblem(w, http.StatusBadRequest, "validation_failed", "expected multipart form data with a file field")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "a file field is required")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, attachments.MaxBytes+1))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "could not read the file")
		return
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	meta, err := h.svc.Upload(r.Context(), orgID, r.PathValue("project"), r.PathValue("task"), userID, header.Filename, contentType, data)
	if err != nil {
		h.fail(w, "attachments.upload", err)
		return
	}
	writeJSON(w, http.StatusCreated, attachmentJSON(meta))
}

func (h *attachmentHandlers) download(w http.ResponseWriter, r *http.Request) {
	orgID, _, ok := h.authorize(w, r, "viewer")
	if !ok {
		return
	}
	item, err := h.svc.Get(r.Context(), orgID, r.PathValue("task"), r.PathValue("attachment"))
	if err != nil {
		h.fail(w, "attachments.get", err)
		return
	}
	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+safeFilename(item.Filename)+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(item.Data)
}

func (h *attachmentHandlers) fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "attachment not found")
	case errors.Is(err, domain.ErrValidation):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		h.log.Error(op, slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not process the attachment")
	}
}

func attachmentJSON(item attachments.Meta) map[string]any {
	return map[string]any{
		"id": item.ID, "filename": item.Filename, "content_type": item.ContentType,
		"size_bytes": item.SizeBytes, "sha256": item.SHA256, "created_at": item.CreatedAt,
	}
}

func safeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r == '\n' || r == '\r' {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
