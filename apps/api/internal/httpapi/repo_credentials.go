package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/repocreds"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// RepoCredentialService stores the per-repository git credential.
type RepoCredentialService interface {
	Set(ctx context.Context, orgID, repoID, actorID, token string) (repocreds.Status, error)
	Delete(ctx context.Context, orgID, repoID, actorID string) error
	Status(ctx context.Context, orgID, repoID string) (repocreds.Status, error)
	Token(ctx context.Context, orgID, repoID string) (string, error)
}

// credential returns whether a repository has its own git credential.
func (h *repoHandlers) credential(w http.ResponseWriter, r *http.Request) {
	repo, claims, ok := h.repoForCredential(w, r, "member")
	if !ok {
		return
	}
	st, err := h.creds.Status(r.Context(), claims.OrgID, repo.ID)
	if err != nil {
		h.log.Error("repos.credential.get", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not read the credential")
		return
	}
	writeJSON(w, http.StatusOK, credentialJSON(st))
}

// setCredential encrypts and stores a token for one repository.
func (h *repoHandlers) setCredential(w http.ResponseWriter, r *http.Request) {
	repo, claims, ok := h.repoForCredential(w, r, "admin")
	if !ok {
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if !decode(w, r, 1<<20, &req) {
		return
	}
	st, err := h.creds.Set(r.Context(), claims.OrgID, repo.ID, claims.Subject, req.Token)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		h.log.Error("repos.credential.set", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not store the credential")
		return
	}
	writeJSON(w, http.StatusOK, credentialJSON(st))
}

// deleteCredential removes the repository credential and falls back to the
// organisation / global token.
func (h *repoHandlers) deleteCredential(w http.ResponseWriter, r *http.Request) {
	repo, claims, ok := h.repoForCredential(w, r, "admin")
	if !ok {
		return
	}
	if err := h.creds.Delete(r.Context(), claims.OrgID, repo.ID, claims.Subject); err != nil {
		h.log.Error("repos.credential.delete", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not remove the credential")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// repoForCredential authenticates the caller, enforces the minimum role and
// resolves the repo inside the project from the path.
func (h *repoHandlers) repoForCredential(w http.ResponseWriter, r *http.Request, minimum string) (domain.Repo, *auth.Claims, bool) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok || r.PathValue("org") != claims.OrgID {
		writeProblem(w, http.StatusNotFound, "not_found", "resource not found")
		return domain.Repo{}, nil, false
	}
	if !roleAtLeast(claims.Role, minimum) {
		writeProblem(w, http.StatusForbidden, "forbidden", "insufficient role")
		return domain.Repo{}, nil, false
	}
	repo, err := h.repos.Get(r.Context(), claims.OrgID, r.PathValue("project"), r.PathValue("repo"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "repository not found")
			return domain.Repo{}, nil, false
		}
		h.log.Error("repos.credential.lookup", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not load the repository")
		return domain.Repo{}, nil, false
	}
	return repo, claims, true
}

// credentialJSON never includes the token; hint is the last four characters.
func credentialJSON(st repocreds.Status) map[string]any {
	out := map[string]any{"configured": st.Configured, "hint": st.Hint, "updated_at": nil}
	if st.UpdatedAt != nil {
		out["updated_at"] = st.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return out
}
