package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	crewsaml "github.com/crewjam/saml"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/saml"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/replay"
)

// SAMLAccounts signs in a verified SAML identity using the OIDC account path.
type SAMLAccounts interface {
	OIDCLogin(ctx context.Context, id auth.OIDCIdentity, platform auth.Platform) (auth.TokenPair, error)
}

// SAMLSP is the SP surface used by the handlers.
type SAMLSP interface {
	Metadata() ([]byte, error)
	LoginURL() (string, string, error)
	Parse(r *http.Request, requestID string) (*crewsaml.Assertion, error)
	PostLogout(nameID string) ([]byte, error)
	WebURL() string
}

type samlHandlers struct {
	svc         SAMLSP
	accounts    SAMLAccounts
	adminGroups map[string]bool
	roleMap     map[string]string
	log         *slog.Logger
	replay      replay.Cache
}

func (h *samlHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /saml/metadata", h.metadata)
	mux.HandleFunc("GET /saml/login", h.login)
	mux.HandleFunc("POST /saml/acs", h.acs)
	mux.HandleFunc("GET /saml/logout", h.logout)
	mux.HandleFunc("GET /saml/slo", h.slo)
	mux.HandleFunc("POST /saml/slo", h.slo)
}

func (h *samlHandlers) metadata(w http.ResponseWriter, _ *http.Request) {
	body, err := h.svc.Metadata()
	if err != nil {
		http.Error(w, "metadata", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write(body)
}

func (h *samlHandlers) login(w http.ResponseWriter, r *http.Request) {
	target, _, err := h.svc.LoginURL()
	if err != nil {
		h.log.Error("saml.login", slog.Any("err", err))
		http.Redirect(w, r, h.svc.WebURL()+"/login?error=saml", http.StatusFound)
		return
	}
	http.Redirect(w, r, target, http.StatusFound) //nolint:gosec // target is the trusted IdP SLO URL from metadata
}

func (h *samlHandlers) acs(w http.ResponseWriter, r *http.Request) {
	// crewjam/saml can panic on a signed assertion without <Issuer> (GO-2026-6909).
	// Recover here so a malformed IdP response cannot crash the API process.
	defer func() {
		if recovered := recover(); recovered != nil {
			h.log.Error("saml.acs panic", slog.Any("panic", recovered))
			h.fail(w, r, "invalid assertion")
		}
	}()
	requestID := r.FormValue("RelayState")
	if requestID == "" {
		h.fail(w, r, "missing RelayState")
		return
	}
	assertion, err := h.svc.Parse(r, requestID)
	if err != nil {
		h.log.Warn("saml.acs parse", slog.Any("err", err))
		h.fail(w, r, err.Error())
		return
	}
	if h.replay != nil {
		seen, rerr := h.replay.Seen(r.Context(), assertion.ID, assertionExpiry(assertion))
		if rerr != nil {
			h.log.Error("saml.replay", slog.Any("err", rerr))
			h.fail(w, r, "replay check failed")
			return
		}
		if seen {
			h.fail(w, r, "replayed assertion")
			return
		}
	}
	subject, email, name, groups, err := saml.Identity(assertion)
	if err != nil {
		h.fail(w, r, err.Error())
		return
	}
	role := h.roleFor(groups)
	pair, err := h.accounts.OIDCLogin(r.Context(), auth.OIDCIdentity{
		Provider: "saml", Subject: subject, Email: email, Name: name, Groups: groups, RoleOverride: role,
	}, auth.PlatformWeb)
	if err != nil {
		h.fail(w, r, "account mapping failed")
		return
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // name id is not a secret; HttpOnly, lax
		Name: "aip_saml_nameid", Value: subject, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 12 * 3600,
	})
	fragment := "access=" + url.QueryEscape(pair.AccessToken) + "&refresh=" + url.QueryEscape(pair.RefreshToken)
	http.Redirect(w, r, h.svc.WebURL()+"/auth/callback#"+fragment, http.StatusFound)
}

func (h *samlHandlers) fail(w http.ResponseWriter, r *http.Request, reason string) {
	h.log.Warn("saml.acs", slog.String("reason", reason))
	http.Redirect(w, r, h.svc.WebURL()+"/login?error=saml", http.StatusFound)
}

func assertionExpiry(assertion *crewsaml.Assertion) time.Time {
	expiry := time.Now().Add(5 * time.Minute)
	if assertion.Conditions != nil && !assertion.Conditions.NotOnOrAfter.IsZero() {
		expiry = assertion.Conditions.NotOnOrAfter
	}
	return expiry
}

func (h *samlHandlers) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("aip_saml_nameid")
	if err != nil || cookie.Value == "" {
		http.Redirect(w, r, h.svc.WebURL()+"/login", http.StatusFound)
		return
	}
	body, err := h.svc.PostLogout(cookie.Value)
	if err != nil {
		h.log.Warn("saml.logout", slog.Any("err", err))
		http.Redirect(w, r, h.svc.WebURL()+"/login", http.StatusFound)
		return
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // clearing; same flags
		Name: "aip_saml_nameid", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body) //nolint:gosec // auto-submitting form generated from trusted IdP metadata
}

func (h *samlHandlers) slo(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // clearing; same flags
		Name: "aip_saml_nameid", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.svc.WebURL()+"/login?logged_out=1", http.StatusFound)
}

func (h *samlHandlers) roleFor(groups []string) string {
	for _, group := range groups {
		if role, ok := h.roleMap[group]; ok {
			switch role {
			case "admin", "member", "viewer":
				return role
			}
		}
	}
	for _, group := range groups {
		if h.adminGroups[group] {
			return "admin"
		}
	}
	return ""
}
