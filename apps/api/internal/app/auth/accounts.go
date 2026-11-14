package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// ErrNoOrganization means the account exists but belongs to no org yet.
var ErrNoOrganization = errors.New("account has no organization")

// AccountStore is the persistence port for accounts and one-time tokens.
type AccountStore interface {
	CreateAccount(ctx context.Context, p CreateAccountParams) (domain.User, domain.Org, error)
	OrgSlugExists(ctx context.Context, slug string) (bool, error)
	UserByEmail(ctx context.Context, email string) (domain.User, string, error)
	MarkEmailVerified(ctx context.Context, userID string, at time.Time) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	PrimaryMembership(ctx context.Context, userID string) (orgID, role string, err error)
	SetMembershipRole(ctx context.Context, userID, orgID, role string) error
	CreateVerification(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error
	ConsumeVerification(ctx context.Context, tokenHash []byte, at time.Time) (string, error)
	CreatePasswordReset(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time, ip string) error
	ConsumePasswordReset(ctx context.Context, tokenHash []byte, at time.Time) (string, error)
	RecordAudit(ctx context.Context, ev AuditEvent) error
	UserIDByIdentity(ctx context.Context, provider, subject string) (string, error)
	LinkIdentity(ctx context.Context, provider, subject, userID string) error
	CreateUser(ctx context.Context, email, name, passwordHash string, verifiedAt time.Time) (domain.User, error)
	AddMembership(ctx context.Context, orgID, userID, role string) error
}

// CreateAccountParams creates a user, an org and the owner membership atomically.
type CreateAccountParams struct {
	Email        string
	Name         string
	PasswordHash string
	OrgName      string
	OrgSlug      string
	Verified     bool
}

// AuditEvent is one append-only audit record.
type AuditEvent struct {
	OrgID    string
	ActorID  string
	Actor    string // user | api_key | system
	Action   string
	Target   string
	Metadata map[string]any
}

// Mailer sends transactional mail. Implementations must not log tokens.
type Mailer interface {
	SendVerification(ctx context.Context, to, link string) error
	SendPasswordReset(ctx context.Context, to, link string) error
}

// Accounts implements registration, sign-in and password recovery.
type Accounts struct {
	store  AccountStore
	tokens *Service
	mailer Mailer
	now    func() time.Time
	appURL string
	verify time.Duration
	reset  time.Duration
	log    *slog.Logger
	dummy  string
}

// NewAccounts wires the account service.
func NewAccounts(store AccountStore, tokens *Service, mailer Mailer, appURL string, log *slog.Logger) (*Accounts, error) {
	if store == nil || tokens == nil || mailer == nil {
		return nil, errors.New("auth: accounts needs store, tokens and mailer")
	}
	dummy, err := HashPassword("dummy password for timing", DefaultParams())
	if err != nil {
		return nil, err
	}
	return &Accounts{
		store:  store,
		tokens: tokens,
		mailer: mailer,
		now:    time.Now,
		appURL: strings.TrimRight(appURL, "/"),
		verify: 24 * time.Hour,
		reset:  time.Hour,
		log:    log,
		dummy:  dummy,
	}, nil
}

// RegisterInput is the self-serve sign-up payload.
type RegisterInput struct {
	Email    string
	Name     string
	Password string
	OrgName  string
}

// RegisterResult front-loads nothing secret; the verification token goes by email.
type RegisterResult struct {
	UserID   string
	OrgSlug  string
	Verified bool
}

// Register creates the account, the org and the owner membership, then sends a
// verification link. It never reveals whether the email already exists.
func (a *Accounts) Register(ctx context.Context, in RegisterInput) (RegisterResult, error) {
	user, err := domain.NewUser(in.Email, in.Name)
	if err != nil {
		return RegisterResult{}, err
	}
	if err := ValidatePassword(in.Password, user.Email); err != nil {
		return RegisterResult{}, err
	}
	orgName := strings.TrimSpace(in.OrgName)
	if orgName == "" {
		orgName = user.Name + "'s workspace"
	}
	org, err := domain.NewOrg(orgName, domain.Slugify(orgName))
	if err != nil {
		return RegisterResult{}, err
	}

	hash, err := HashPassword(in.Password, DefaultParams())
	if err != nil {
		return RegisterResult{}, err
	}

	slug := domain.UniqueSlug(org.Slug, func(candidate string) bool {
		taken, err := a.store.OrgSlugExists(ctx, candidate)
		return err == nil && taken
	})
	created, createdOrg, err := a.store.CreateAccount(ctx, CreateAccountParams{
		Email:        user.Email,
		Name:         user.Name,
		PasswordHash: hash,
		OrgName:      org.Name,
		OrgSlug:      slug,
	})
	if err != nil {
		// Unique violation on email or slug is reported as a conflict; the HTTP
		// layer turns it into the same 202 as success to avoid enumeration.
		return RegisterResult{}, err
	}

	raw, tokenHash, err := newRefreshToken()
	if err != nil {
		return RegisterResult{}, err
	}
	if err := a.store.CreateVerification(ctx, created.ID, tokenHash, a.now().UTC().Add(a.verify)); err != nil {
		return RegisterResult{}, fmt.Errorf("auth: create verification: %w", err)
	}
	if err := a.mailer.SendVerification(ctx, created.Email, a.appURL+"/verify?token="+raw); err != nil {
		a.log.Warn("auth: verification email failed", slog.String("user_id", created.ID), slog.Any("err", err))
	}
	a.audit(ctx, AuditEvent{OrgID: createdOrg.ID, ActorID: created.ID, Actor: "user",
		Action: "auth.register", Target: "user:" + created.ID})

	return RegisterResult{UserID: created.ID, OrgSlug: createdOrg.Slug, Verified: false}, nil
}

// VerifyEmail consumes the one-time token and marks the account verified.
func (a *Accounts) VerifyEmail(ctx context.Context, token string) error {
	userID, err := a.store.ConsumeVerification(ctx, hashToken(token), a.now().UTC())
	if err != nil {
		return ErrInvalidToken
	}
	if err := a.store.MarkEmailVerified(ctx, userID, a.now().UTC()); err != nil {
		return fmt.Errorf("auth: mark verified: %w", err)
	}
	a.audit(ctx, AuditEvent{ActorID: userID, Actor: "user", Action: "auth.email.verified", Target: "user:" + userID})
	return nil
}

// LoginInput is the sign-in payload.
type LoginInput struct {
	Email    string
	Password string
	Platform Platform
}

// Login verifies credentials and issues a session. Unknown emails take the same
// code path (and cost) as known ones with a wrong password.
func (a *Accounts) Login(ctx context.Context, in LoginInput) (TokenPair, error) {
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	user, passwordHash, err := a.store.UserByEmail(ctx, email)
	if err != nil {
		_, _ = VerifyPassword(in.Password, a.dummy)
		return TokenPair{}, ErrInvalidCredentials
	}

	result, err := VerifyPassword(in.Password, passwordHash)
	if err != nil || !result.Valid {
		a.audit(ctx, AuditEvent{ActorID: user.ID, Actor: "user", Action: "auth.login.failed", Target: "user:" + user.ID})
		return TokenPair{}, ErrInvalidCredentials
	}
	if result.NeedsRehash {
		if hash, err := HashPassword(in.Password, DefaultParams()); err == nil {
			_ = a.store.UpdatePassword(ctx, user.ID, hash)
		}
	}

	orgID, role, err := a.store.PrimaryMembership(ctx, user.ID)
	if err != nil || orgID == "" {
		return TokenPair{}, ErrNoOrganization
	}

	pair, err := a.tokens.Issue(ctx, IssueRequest{
		UserID: user.ID, OrgID: orgID, Role: role, Platform: in.Platform,
	})
	if err != nil {
		return TokenPair{}, err
	}
	a.audit(ctx, AuditEvent{OrgID: orgID, ActorID: user.ID, Actor: "user",
		Action: "auth.login.succeeded", Target: "user:" + user.ID})
	return pair, nil
}

// OIDCIdentity is the verified identity returned by an OIDC provider.
type OIDCIdentity struct {
	Provider     string
	Subject      string
	Email        string
	Name         string
	Groups       []string
	RoleOverride string
}

// OIDCLogin signs in (or provisions) a user from a verified OIDC identity.
func (a *Accounts) OIDCLogin(ctx context.Context, id OIDCIdentity, platform Platform) (TokenPair, error) {
	userID, err := a.store.UserIDByIdentity(ctx, id.Provider, id.Subject)
	if err != nil {
		email, normErr := domain.NormalizeEmail(id.Email)
		if normErr != nil {
			return TokenPair{}, ErrInvalidCredentials
		}
		if existing, _, uerr := a.store.UserByEmail(ctx, email); uerr == nil {
			userID = existing.ID
		} else {
			hash, herr := HashPassword(randomPassword(), DefaultParams())
			if herr != nil {
				return TokenPair{}, herr
			}
			orgName := strings.TrimSpace(id.Name) + "'s workspace"
			if strings.TrimSpace(id.Name) == "" {
				orgName = "My workspace"
			}
			org, oerr := domain.NewOrg(orgName, domain.Slugify(orgName))
			if oerr != nil {
				return TokenPair{}, oerr
			}
			slug := domain.UniqueSlug(org.Slug, func(candidate string) bool {
				taken, serr := a.store.OrgSlugExists(ctx, candidate)
				return serr == nil && taken
			})
			created, _, cerr := a.store.CreateAccount(ctx, CreateAccountParams{
				Email: email, Name: strings.TrimSpace(id.Name), PasswordHash: hash,
				OrgName: org.Name, OrgSlug: slug,
			})
			if cerr != nil {
				return TokenPair{}, cerr
			}
			userID = created.ID
			_ = a.store.MarkEmailVerified(ctx, userID, a.now().UTC())
		}
		if lerr := a.store.LinkIdentity(ctx, id.Provider, id.Subject, userID); lerr != nil {
			return TokenPair{}, lerr
		}
	}

	orgID, role, err := a.store.PrimaryMembership(ctx, userID)
	if err != nil || orgID == "" {
		return TokenPair{}, ErrNoOrganization
	}
	if id.RoleOverride != "" && id.RoleOverride != role && role != "owner" {
		if id.RoleOverride != "admin" && id.RoleOverride != "member" && id.RoleOverride != "viewer" {
			return TokenPair{}, ErrInvalidCredentials
		}
		if serr := a.store.SetMembershipRole(ctx, userID, orgID, id.RoleOverride); serr != nil {
			return TokenPair{}, serr
		}
		role = id.RoleOverride
	}
	pair, err := a.tokens.Issue(ctx, IssueRequest{
		UserID: userID, OrgID: orgID, Role: role, Platform: platform,
	})
	if err != nil {
		return TokenPair{}, err
	}
	a.audit(ctx, AuditEvent{OrgID: orgID, ActorID: userID, Actor: "user",
		Action: "auth.login.oidc", Target: "user:" + userID})
	return pair, nil
}

// CreateInvitedUser creates a user from an accepted invite and signs them in.
func (a *Accounts) CreateInvitedUser(ctx context.Context, email, name, password, orgID, role string) (TokenPair, error) {
	user, err := domain.NewUser(email, name)
	if err != nil {
		return TokenPair{}, err
	}
	if err := ValidatePassword(password, user.Email); err != nil {
		return TokenPair{}, err
	}
	hash, err := HashPassword(password, DefaultParams())
	if err != nil {
		return TokenPair{}, err
	}
	created, err := a.store.CreateUser(ctx, user.Email, user.Name, hash, a.now().UTC())
	if err != nil {
		return TokenPair{}, err
	}
	if err := a.store.AddMembership(ctx, orgID, created.ID, role); err != nil {
		return TokenPair{}, err
	}
	pair, err := a.tokens.Issue(ctx, IssueRequest{UserID: created.ID, OrgID: orgID, Role: role, Platform: PlatformWeb})
	if err != nil {
		return TokenPair{}, err
	}
	a.audit(ctx, AuditEvent{OrgID: orgID, ActorID: created.ID, Actor: "user",
		Action: "auth.invite.accepted", Target: "user:" + created.ID})
	return pair, nil
}

// randomPassword returns an unusable random secret for OIDC-only accounts.
func randomPassword() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "oidc-" + time.Now().UTC().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(buf)
}

// Refresh rotates the refresh token (delegates to the token service).
func (a *Accounts) Refresh(ctx context.Context, raw string, platform Platform) (TokenPair, error) {
	return a.tokens.Refresh(ctx, raw, platform)
}

// LogoutAll revokes every session of the user.
func (a *Accounts) LogoutAll(ctx context.Context, userID string) error {
	return a.tokens.RevokeAllForUser(ctx, userID, "logout_all")
}

// Logout revokes the session.
func (a *Accounts) Logout(ctx context.Context, sessionID string) error {
	return a.tokens.Revoke(ctx, sessionID, "logout")
}

// RequestPasswordReset always succeeds from the caller's point of view; it only
// sends mail when the account exists.
func (a *Accounts) RequestPasswordReset(ctx context.Context, email, ip string) error {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return nil
	}
	user, _, err := a.store.UserByEmail(ctx, normalized)
	if err != nil {
		return nil
	}
	raw, tokenHash, err := newRefreshToken()
	if err != nil {
		return err
	}
	if err := a.store.CreatePasswordReset(ctx, user.ID, tokenHash, a.now().UTC().Add(a.reset), ip); err != nil {
		return fmt.Errorf("auth: create password reset: %w", err)
	}
	if err := a.mailer.SendPasswordReset(ctx, user.Email, a.appURL+"/reset?token="+raw); err != nil {
		a.log.Warn("auth: reset email failed", slog.String("user_id", user.ID), slog.Any("err", err))
	}
	a.audit(ctx, AuditEvent{ActorID: user.ID, Actor: "user",
		Action: "auth.password.reset_requested", Target: "user:" + user.ID})
	return nil
}

// ResetPassword consumes the token, stores the new hash and revokes every session.
func (a *Accounts) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := a.store.ConsumePasswordReset(ctx, hashToken(token), a.now().UTC())
	if err != nil {
		return ErrInvalidToken
	}
	if err := ValidatePassword(newPassword, ""); err != nil {
		return err
	}
	hash, err := HashPassword(newPassword, DefaultParams())
	if err != nil {
		return err
	}
	if err := a.store.UpdatePassword(ctx, userID, hash); err != nil {
		return fmt.Errorf("auth: update password: %w", err)
	}
	if err := a.tokens.RevokeAllForUser(ctx, userID, "password reset"); err != nil {
		return fmt.Errorf("auth: revoke sessions: %w", err)
	}
	a.audit(ctx, AuditEvent{ActorID: userID, Actor: "user", Action: "auth.password.reset", Target: "user:" + userID})
	return nil
}

func (a *Accounts) audit(ctx context.Context, ev AuditEvent) {
	if ev.Metadata == nil {
		ev.Metadata = map[string]any{}
	}
	if err := a.store.RecordAudit(ctx, ev); err != nil {
		a.log.Warn("auth: audit write failed", slog.String("action", ev.Action), slog.Any("err", err))
	}
}
