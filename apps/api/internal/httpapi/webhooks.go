package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/git"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/checks"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// MergeMarker moves a reviewed task to done.
type MergeMarker interface {
	MarkMerged(ctx context.Context, orgID, projectID, idOrRef string) (domain.Task, error)
}

// ProviderFactory builds a git provider for a repo's secret.
type ProviderFactory func(provider, secret string) git.Provider

// DefaultProviders constructs the real adapters.
func DefaultProviders(provider, secret string) git.Provider {
	if provider == string(domain.ProviderGitLab) {
		return git.NewGitLab("", secret)
	}
	return git.NewGitHub("", secret)
}

// CheckRecorder stores CI check results reported by providers.
type CheckRecorder interface {
	Record(ctx context.Context, orgID, repoID, prID, runID string, c checks.Check) (checks.Check, error)
	Failed(ctx context.Context, orgID, prID string) (bool, error)
}

type webhookHandlers struct {
	repos     RepoService
	tasks     MergeMarker
	checks    CheckRecorder
	providers ProviderFactory
	appSecret string
	apps      GitHubAppSecretReader
	log       *slog.Logger
	mu        sync.Mutex
	seen      map[string]time.Time
}

func newWebhookHandlers(repos RepoService, tasks MergeMarker, checkRecorder CheckRecorder, providers ProviderFactory, log *slog.Logger) *webhookHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if providers == nil {
		providers = DefaultProviders
	}
	return &webhookHandlers{repos: repos, tasks: tasks, checks: checkRecorder, providers: providers, log: log,
		seen: make(map[string]time.Time)}
}

// GitHubAppSecretReader returns a per-org app webhook secret.
type GitHubAppSecretReader interface {
	WebhookSecret(ctx context.Context, orgID string) (string, error)
}

func (h *webhookHandlers) withGitHubApps(apps GitHubAppSecretReader) *webhookHandlers {
	h.apps = apps
	return h
}

func (h *webhookHandlers) withAppSecret(secret string) *webhookHandlers {
	h.appSecret = secret
	return h
}

func (h *webhookHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhooks/github/{repo}", h.handle(string(domain.ProviderGitHub)))
	mux.HandleFunc("POST /webhooks/github-app", h.handleApp)
	mux.HandleFunc("POST /webhooks/github-app/{org}", h.handleOrgApp)
	mux.HandleFunc("POST /webhooks/gitlab/{repo}", h.handle(string(domain.ProviderGitLab)))
	mux.HandleFunc("POST /webhooks/gitea/{repo}", h.handle(string(domain.ProviderGitea)))
	mux.HandleFunc("POST /webhooks/bitbucket/{repo}", h.handle(string(domain.ProviderBitbucket)))
}

func (h *webhookHandlers) handle(expected string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", "webhook payload is too large")
			return
		}

		repo, err := h.repos.ForWebhook(r.Context(), r.PathValue("repo"))
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeProblem(w, http.StatusNotFound, "not_found", "repository not found")
				return
			}
			h.log.Error("webhooks.repo", slog.Any("err", err))
			writeProblem(w, http.StatusInternalServerError, "internal", "could not resolve the repository")
			return
		}
		if string(repo.Provider) != expected {
			writeProblem(w, http.StatusNotFound, "not_found", "repository not found")
			return
		}

		event, err := h.providers(expected, repo.WebhookSecret).ParseWebhook(r, body)
		if err != nil {
			h.log.Warn("webhooks.verify", slog.String("provider", expected), slog.Any("err", err))
			writeProblem(w, http.StatusUnauthorized, "invalid_signature", "webhook verification failed")
			return
		}

		// Replay protection: a signed delivery may be retried by the provider or
		// replayed by an attacker who captured it. Keep delivery ids for 10 min
		// (in-memory, per replica; the DB is still idempotent for checks/merges).
		if delivery := r.Header.Get("X-GitHub-Delivery") + r.Header.Get("X-Gitlab-Event-UUID"); delivery != "" {
			if h.duplicate(delivery) {
				writeJSON(w, http.StatusAccepted, map[string]any{"status": "duplicate"})
				return
			}
		}

		status := h.processEvent(r.Context(), repo, event)
		writeJSON(w, http.StatusAccepted, map[string]any{"status": status})
	}
}

func (h *webhookHandlers) handleApp(w http.ResponseWriter, r *http.Request) {
	if h.appSecret == "" || h.repos == nil {
		writeProblem(w, http.StatusNotFound, "not_found", "github app webhook is not configured")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", "webhook payload is too large")
		return
	}
	event, err := h.providers("github", h.appSecret).ParseWebhook(r, body)
	if err != nil {
		h.log.Warn("webhooks.app_verify", slog.Any("err", err))
		writeProblem(w, http.StatusUnauthorized, "invalid_signature", "webhook verification failed")
		return
	}
	if delivery := r.Header.Get("X-GitHub-Delivery"); delivery != "" && h.duplicate(delivery) {
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "duplicate"})
		return
	}
	repo, err := h.repos.ForWebhookName(r.Context(), "github", event.Repo)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "not_found", "repository not found")
			return
		}
		h.log.Error("webhooks.app_repo", slog.Any("err", err))
		writeProblem(w, http.StatusInternalServerError, "internal", "could not resolve the repository")
		return
	}
	status := h.processEvent(r.Context(), repo, event)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": status})
}

func (h *webhookHandlers) handleOrgApp(w http.ResponseWriter, r *http.Request) {
	if h.apps == nil || h.repos == nil {
		writeProblem(w, http.StatusNotFound, "not_found", "github app webhook is not configured")
		return
	}
	orgID := r.PathValue("org")
	if !isUUID(orgID) {
		writeProblem(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge, "payload_too_large", "webhook payload is too large")
		return
	}
	secret, err := h.apps.WebhookSecret(r.Context(), orgID)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "github app is not configured")
		return
	}
	event, err := h.providers("github", secret).ParseWebhook(r, body)
	if err != nil {
		writeProblem(w, http.StatusUnauthorized, "invalid_signature", "webhook verification failed")
		return
	}
	repo, err := h.repos.(interface {
		ForWebhookNameInOrg(ctx context.Context, orgID, provider, name string) (domain.Repo, error)
	}).ForWebhookNameInOrg(r.Context(), orgID, "github", event.Repo)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}
	status := h.processEvent(r.Context(), repo, event)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": status})
}

// processEvent records a check or a merged PR for an already resolved repo.
func (h *webhookHandlers) processEvent(ctx context.Context, repo domain.Repo, event git.WebhookEvent) string {
	if event.Kind == "check_run" {
		if event.Number > 0 && h.checks != nil {
			if pr, perr := h.repos.PullRequestForMerge(ctx, repo.OrgID, repo.ID, event.Number); perr == nil {
				if _, cerr := h.checks.Record(ctx, repo.OrgID, repo.ID, pr.ID, "", checks.Check{
					Name: event.CheckName, Status: event.CheckStatus, Output: event.CheckOutput,
					DurationMS: event.CheckDurationMS, CommitSHA: event.CommitSHA, URL: event.CheckURL,
				}); cerr != nil {
					h.log.Warn("webhooks.check", slog.Any("err", cerr))
				} else {
					return "check_recorded"
				}
			}
		}
		return "ignored"
	}
	if event.Merged {
		pr, perr := h.repos.PullRequestForMerge(ctx, repo.OrgID, repo.ID, event.Number)
		if perr == nil {
			approved, aerr := h.repos.ApprovedForPR(ctx, repo.OrgID, pr.ID)
			if aerr != nil || !approved {
				return "approval_required"
			}
			if _, terr := h.tasks.MarkMerged(ctx, repo.OrgID, repo.ProjectID, pr.TaskID); terr != nil {
				h.log.Warn("webhooks.mark_merged", slog.Any("err", terr))
			}
			_ = h.repos.SetPullRequestState(ctx, repo.OrgID, pr.ID, "merged")
			return "merged"
		}
	}
	return "accepted"
}

// duplicate records a delivery id and reports whether it was seen recently.
func (h *webhookHandlers) duplicate(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if seenAt, ok := h.seen[id]; ok && now.Sub(seenAt) < 10*time.Minute {
		return true
	}
	for key, seenAt := range h.seen {
		if now.Sub(seenAt) >= 10*time.Minute {
			delete(h.seen, key)
		}
	}
	h.seen[id] = now
	return false
}

// isUUID is a cheap shape check for org path values.
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !isHexDigit(r) {
			return false
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
