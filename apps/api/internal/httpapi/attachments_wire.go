package httpapi

import (
	"log/slog"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
)

// authIssuer aliases the concrete issuer used by other handlers.
type authIssuer = auth.Issuer

func newAttachmentHandlers(svc AttachmentService, issuer *auth.Issuer, log *slog.Logger) *attachmentHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &attachmentHandlers{svc: svc, issuer: issuer, log: log}
}
