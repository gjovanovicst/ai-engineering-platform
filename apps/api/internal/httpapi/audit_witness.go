package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auditwitness"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// AuditWitnessService anchors and verifies the audit chain.
type AuditWitnessService interface {
	Create(ctx context.Context, orgID string) (auditwitness.Witness, error)
	Verify(ctx context.Context, orgID string) (auditwitness.Witness, bool, error)
}

type auditWitnessHandlers struct {
	svc    AuditWitnessService
	issuer *auth.Issuer
	log    *slog.Logger
}

func (h *auditWitnessHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orgs/{org}/audit/witness", h.create)
	mux.HandleFunc("GET /api/v1/orgs/{org}/audit/witness", h.verify)
}

func (h *auditWitnessHandlers) authorize(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	if !roleAtLeast(claims.Role, "admin") {
		writeProblem(w, http.StatusForbidden, "forbidden", "admin role is required")
		return "", false
	}
	return claims.OrgID, true
}

func witnessJSON(witness auditwitness.Witness) map[string]any {
	return map[string]any{
		"id": witness.ID, "object_key": witness.ObjectKey, "row_count": witness.RowCount,
		"last_id": witness.LastID, "last_hash": witness.LastHash, "created_at": witness.CreatedAt,
	}
}

func (h *auditWitnessHandlers) create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	witness, err := h.svc.Create(r.Context(), orgID)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("audit.witness", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not write the witness")
		return
	}
	writeJSON(w, http.StatusCreated, witnessJSON(witness))
}

func (h *auditWitnessHandlers) verify(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.authorize(w, r)
	if !ok {
		return
	}
	witness, valid, err := h.svc.Verify(r.Context(), orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "no witness has been written")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "internal", "could not verify the witness")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"witness": witnessJSON(witness), "valid": valid})
}
