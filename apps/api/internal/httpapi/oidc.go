package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/oidc"
)

const oidcStateCookie = "aip_oidc_state"

// OIDCService runs the authorization-code flow.
type OIDCService interface {
	Enabled(name string) bool
	Start(name string) (url, state string, err error)
	Callback(ctx context.Context, name, code string) (auth.TokenPair, error)
}

type oidcHandlers struct {
	svc    OIDCService
	webURL string
	log    *slog.Logger
}

func newOIDCHandlers(svc OIDCService, webURL string, log *slog.Logger) *oidcHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &oidcHandlers{svc: svc, webURL: strings.TrimRight(webURL, "/"), log: log}
}

func (h *oidcHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/oidc/{provider}/start", h.start)
	mux.HandleFunc("GET /api/v1/auth/oidc/{provider}/callback", h.callback)
}

func (h *oidcHandlers) start(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	if !h.svc.Enabled(provider) {
		writeProblem(w, http.StatusNotFound, "not_found", "provider is not configured")
		return
	}
	target, state, err := h.svc.Start(provider)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "could not start the login")
		return
	}
	//nolint:gosec // dev runs over http; HttpOnly+SameSite are set, Secure is added behind TLS
	http.SetCookie(w, &http.Cookie{
		Name: oidcStateCookie, Value: state, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int((10 * time.Minute).Seconds()),
	})
	//nolint:gosec // target comes from the trusted provider registry, not the request
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *oidcHandlers) callback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		writeProblem(w, http.StatusBadRequest, "invalid_state", "the login state does not match")
		return
	}
	//nolint:gosec // clearing the state cookie; flags match the cookie above
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeProblem(w, http.StatusBadRequest, "validation_failed", "missing authorization code")
		return
	}
	pair, err := h.svc.Callback(r.Context(), provider, code)
	if err != nil {
		if errors.Is(err, oidc.ErrProviderDisabled) {
			writeProblem(w, http.StatusNotFound, "not_found", "provider is not configured")
			return
		}
		h.log.Warn("oidc.callback", slog.String("provider", provider), slog.Any("err", err))
		http.Redirect(w, r, h.webURL+"/login?error=oidc", http.StatusFound)
		return
	}
	// The fragment is never sent to a server; the page stores the tokens.
	fragment := "access=" + pair.AccessToken + "&refresh=" + pair.RefreshToken
	http.Redirect(w, r, h.webURL+"/auth/callback#"+fragment, http.StatusFound)
}
