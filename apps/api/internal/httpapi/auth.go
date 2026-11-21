package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/auth"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
	"github.com/gjovanovicst/ai-engineering-platform/internal/httpapi/oapi"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/ratelimit"
)

// AccountService is the subset of auth.Accounts the HTTP layer needs. Keeping
// it as an interface makes the handlers testable without a database.
type AccountService interface {
	Register(ctx context.Context, in auth.RegisterInput) (auth.RegisterResult, error)
	VerifyEmail(ctx context.Context, token string) error
	Login(ctx context.Context, in auth.LoginInput) (auth.TokenPair, error)
	Refresh(ctx context.Context, raw string, platform auth.Platform) (auth.TokenPair, error)
	Logout(ctx context.Context, sessionID string) error
	LogoutAll(ctx context.Context, userID string) error
	RequestPasswordReset(ctx context.Context, email, ip string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

// RateLimits groups the per-route limiters (docs/12-auth.md §9). A nil limiter
// disables the check, which keeps unit tests deterministic.
type RateLimits struct {
	Login    ratelimit.Allower
	Register ratelimit.Allower
	Reset    ratelimit.Allower
}

type authHandlers struct {
	svc     AccountService
	issuer  *auth.Issuer
	limits  RateLimits
	log     *slog.Logger
	maxBody int64
	now     func() time.Time
}

func newAuthHandlers(svc AccountService, issuer *auth.Issuer, limits RateLimits, log *slog.Logger) *authHandlers {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &authHandlers{svc: svc, issuer: issuer, limits: limits, log: log, maxBody: 1 << 20, now: time.Now}
}

func (h *authHandlers) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/verify-email", h.verifyEmail)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("POST /api/v1/auth/logout-all", h.logoutAll)
	mux.HandleFunc("POST /api/v1/auth/password/reset-request", h.resetRequest)
	mux.HandleFunc("POST /api/v1/auth/password/reset", h.resetPassword)
	mux.HandleFunc("GET /api/v1/auth/me", h.me)
}

func (h *authHandlers) register(w http.ResponseWriter, r *http.Request) {
	if !allow(h.limits.Register, clientIP(r)) {
		writeProblem(w, http.StatusTooManyRequests, "rate_limited", "too many registrations from this address")
		return
	}
	var req oapi.RegisterInput
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	orgName := ""
	if req.OrgName != nil {
		orgName = *req.OrgName
	}
	res, err := h.svc.Register(r.Context(), auth.RegisterInput{
		Email: string(req.Email), Name: req.Name, Password: req.Password, OrgName: orgName,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict):
			// Same status as success, so the endpoint does not reveal whether
			// the email already exists.
			writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
		case errors.Is(err, domain.ErrValidation), errors.Is(err, auth.ErrWeakPassword):
			writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
		default:
			h.fail(w, "register", err)
		}
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id":        res.UserID,
		"org_slug":       res.OrgSlug,
		"email_verified": res.Verified,
	})
}

func (h *authHandlers) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), req.Token); err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			writeProblem(w, http.StatusGone, "invalid_token", "the verification link is invalid or expired")
			return
		}
		h.fail(w, "verify_email", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true})
}

func (h *authHandlers) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string        `json:"email"`
		Password string        `json:"password"`
		Platform auth.Platform `json:"platform"`
	}
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	if !allow(h.limits.Login, clientIP(r)+"|"+strings.ToLower(req.Email)) {
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "rate_limited", "too many sign-in attempts")
		return
	}

	pair, err := h.svc.Login(r.Context(), auth.LoginInput{
		Email: req.Email, Password: req.Password, Platform: req.Platform,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeProblem(w, http.StatusUnauthorized, "unauthenticated", "invalid email or password")
		case errors.Is(err, auth.ErrNoOrganization):
			writeProblem(w, http.StatusForbidden, "no_organization", "the account does not belong to an organization yet")
		case errors.Is(err, auth.ErrInvalidPlatform):
			writeProblem(w, http.StatusBadRequest, "validation_failed", "unknown platform")
		default:
			h.fail(w, "login", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, tokenJSON(pair))
}

func (h *authHandlers) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string        `json:"refresh_token"`
		Platform     auth.Platform `json:"platform"`
	}
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken, req.Platform)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrRefreshReuse):
			writeProblem(w, http.StatusUnauthorized, "refresh_reuse", "the refresh token was already used; all sessions were revoked")
		case errors.Is(err, auth.ErrSessionExpired), errors.Is(err, auth.ErrInvalidToken):
			writeProblem(w, http.StatusUnauthorized, "unauthenticated", "invalid or expired refresh token")
		default:
			h.fail(w, "refresh", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, tokenJSON(pair))
}

func (h *authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if err := h.svc.Logout(r.Context(), claims.SessionID); err != nil {
		h.fail(w, "logout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) resetRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	if !allow(h.limits.Reset, strings.ToLower(req.Email)) {
		w.Header().Set("Retry-After", "3600")
		writeProblem(w, http.StatusTooManyRequests, "rate_limited", "too many reset requests")
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), req.Email, clientIP(r)); err != nil {
		h.fail(w, "password_reset_request", err)
		return
	}
	// Always 202, regardless of whether the account exists.
	w.WriteHeader(http.StatusAccepted)
}

func (h *authHandlers) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decode(w, r, h.maxBody, &req) {
		return
	}
	err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"sessions_revoked": true})
	case errors.Is(err, auth.ErrInvalidToken):
		writeProblem(w, http.StatusGone, "invalid_token", "the reset link is invalid or expired")
	case errors.Is(err, auth.ErrWeakPassword):
		writeProblem(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		h.fail(w, "password_reset", err)
	}
}

func (h *authHandlers) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.claims(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      claims.Subject,
		"org_id":  claims.OrgID,
		"role":    claims.Role,
		"session": claims.SessionID,
	})
}

func (h *authHandlers) claims(r *http.Request) (*auth.Claims, bool) {
	return bearerClaims(h.issuer, r)
}

func tokenJSON(p auth.TokenPair) map[string]any {
	return map[string]any{
		"access_token":       p.AccessToken,
		"refresh_token":      p.RefreshToken,
		"expires_in":         p.ExpiresIn,
		"refresh_expires_in": p.RefreshExpiresIn,
		"token_type":         "Bearer",
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) < 8 || !strings.EqualFold(h[:7], "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(h[7:]), true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func decode(w http.ResponseWriter, r *http.Request, max int64, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, max)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		// Name the unknown field: "invalid JSON body" for {"slug": ...} made the
		// pilot setup guess for an hour.
		detail := "invalid JSON body"
		if strings.Contains(err.Error(), "unknown field") {
			detail = err.Error()
		}
		writeProblem(w, http.StatusBadRequest, "validation_failed", detail)
		return false
	}
	return true
}

func allow(l ratelimit.Allower, key string) bool {
	if l == nil {
		return true
	}
	ok, retry := l.Allow(key)
	_ = retry
	return ok
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://api.aip.dev/errors/" + strings.ReplaceAll(code, "_", "-"),
		"title":  http.StatusText(status),
		"status": status,
		"code":   code,
		"detail": detail,
	})
}

// fail logs the internal cause and returns a generic 500; details never reach
// the client.
func (h *authHandlers) fail(w http.ResponseWriter, op string, err error) {
	h.log.Error("auth."+op, slog.Any("err", err))
	writeProblem(w, http.StatusInternalServerError, "internal", "request failed")
}

func (h *authHandlers) logoutAll(w http.ResponseWriter, r *http.Request) {
	claims, ok := bearerClaims(h.issuer, r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "unauthenticated", "missing or invalid access token")
		return
	}
	if err := h.svc.LogoutAll(r.Context(), claims.Subject); err != nil {
		h.fail(w, "logout all", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
