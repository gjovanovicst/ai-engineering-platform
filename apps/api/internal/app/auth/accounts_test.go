package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeMailer struct {
	verifications []string
	resets        []string
}

func (m *fakeMailer) SendVerification(_ context.Context, _, link string) error {
	m.verifications = append(m.verifications, link)
	return nil
}

func (m *fakeMailer) SendPasswordReset(_ context.Context, _, link string) error {
	m.resets = append(m.resets, link)
	return nil
}

type fakeAccountStore struct {
	users         map[string]domain.User
	hashes        map[string]string
	memberships   map[string][2]string // userID -> {orgID, role}
	verifications map[string]string    // hex(hash) -> userID
	resets        map[string]string
	orgSlugs      map[string]bool
	audits        []AuditEvent
	nextID        int
}

func newFakeAccountStore() *fakeAccountStore {
	return &fakeAccountStore{
		users:         map[string]domain.User{},
		hashes:        map[string]string{},
		memberships:   map[string][2]string{},
		verifications: map[string]string{},
		resets:        map[string]string{},
		orgSlugs:      map[string]bool{},
	}
}

func (f *fakeAccountStore) CreateAccount(_ context.Context, p CreateAccountParams) (domain.User, domain.Org, error) {
	if _, ok := f.users[p.Email]; ok {
		return domain.User{}, domain.Org{}, domain.ErrConflict
	}
	f.nextID++
	id := "user-" + string(rune('0'+f.nextID))
	user := domain.User{ID: id, Email: p.Email, Name: p.Name, EmailVerified: p.Verified}
	org := domain.Org{ID: "org-" + id, Slug: p.OrgSlug, Name: p.OrgName, MaxConcurrentRuns: 4}
	f.users[p.Email] = user
	f.hashes[p.Email] = p.PasswordHash
	f.orgSlugs[p.OrgSlug] = true
	f.memberships[id] = [2]string{org.ID, "owner"}
	return user, org, nil
}

func (f *fakeAccountStore) UserByEmail(_ context.Context, email string) (domain.User, string, error) {
	u, ok := f.users[email]
	if !ok {
		return domain.User{}, "", domain.ErrNotFound
	}
	return u, f.hashes[email], nil
}

func (f *fakeAccountStore) MarkEmailVerified(_ context.Context, userID string, _ time.Time) error {
	for email, u := range f.users {
		if u.ID == userID {
			u.EmailVerified = true
			f.users[email] = u
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeAccountStore) UpdatePassword(_ context.Context, userID, hash string) error {
	for email, u := range f.users {
		if u.ID == userID {
			f.hashes[email] = hash
			return nil
		}
	}
	return domain.ErrNotFound
}

func (f *fakeAccountStore) OrgSlugExists(_ context.Context, slug string) (bool, error) {
	return f.orgSlugs[slug], nil
}

func (f *fakeAccountStore) SetMembershipRole(_ context.Context, userID, orgID, role string) error {
	if f.memberships == nil {
		f.memberships = map[string][2]string{}
	}
	f.memberships[userID] = [2]string{orgID, role}
	return nil
}

func (f *fakeAccountStore) PrimaryMembership(_ context.Context, userID string) (string, string, error) {
	m, ok := f.memberships[userID]
	if !ok {
		return "", "", domain.ErrNotFound
	}
	return m[0], m[1], nil
}

func (f *fakeAccountStore) CreateVerification(_ context.Context, userID string, hash []byte, _ time.Time) error {
	f.verifications[hexString(hash)] = userID
	return nil
}

func (f *fakeAccountStore) ConsumeVerification(_ context.Context, hash []byte, _ time.Time) (string, error) {
	key := hexString(hash)
	id, ok := f.verifications[key]
	if !ok {
		return "", domain.ErrNotFound
	}
	delete(f.verifications, key)
	return id, nil
}

func (f *fakeAccountStore) CreatePasswordReset(_ context.Context, userID string, hash []byte, _ time.Time, _ string) error {
	f.resets[hexString(hash)] = userID
	return nil
}

func (f *fakeAccountStore) ConsumePasswordReset(_ context.Context, hash []byte, _ time.Time) (string, error) {
	key := hexString(hash)
	id, ok := f.resets[key]
	if !ok {
		return "", domain.ErrNotFound
	}
	delete(f.resets, key)
	return id, nil
}

func (f *fakeAccountStore) UserIDByIdentity(_ context.Context, _, _ string) (string, error) {
	return "", domain.ErrNotFound
}

func (f *fakeAccountStore) LinkIdentity(context.Context, string, string, string) error { return nil }

func (f *fakeAccountStore) CreateUser(_ context.Context, email, name, hash string, _ time.Time) (domain.User, error) {
	user := domain.User{ID: "user-" + email, Email: email, Name: name}
	if f.users == nil {
		f.users = map[string]domain.User{}
	}
	f.users[email] = user
	if f.hashes == nil {
		f.hashes = map[string]string{}
	}
	f.hashes[email] = hash
	return user, nil
}

func (f *fakeAccountStore) AddMembership(_ context.Context, orgID, userID, role string) error {
	if f.memberships == nil {
		f.memberships = map[string][2]string{}
	}
	f.memberships[userID] = [2]string{orgID, role}
	return nil
}

func (f *fakeAccountStore) RecordAudit(_ context.Context, ev AuditEvent) error {
	f.audits = append(f.audits, ev)
	return nil
}

func newTestAccounts(t *testing.T) (*Accounts, *fakeAccountStore, *fakeMailer, *memStore) {
	t.Helper()
	store := newFakeAccountStore()
	mail := &fakeMailer{}
	issuer, err := NewIssuer(testPEM(t), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	sessions := newMemStore()
	tokens, err := NewService(sessions, issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(store, tokens, mail, "https://app.test", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return accounts, store, mail, sessions
}

func TestRegisterSendsVerification(t *testing.T) {
	accounts, store, mail, _ := newTestAccounts(t)

	res, err := accounts.Register(context.Background(), RegisterInput{
		Email: "Goran@Acme.IO", Name: "Goran", Password: "correct horse battery", OrgName: "Acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.UserID == "" || res.OrgSlug != "acme" {
		t.Fatalf("result = %+v", res)
	}
	if len(mail.verifications) != 1 || !strings.Contains(mail.verifications[0], "/verify?token=") {
		t.Fatalf("verification mails = %v", mail.verifications)
	}
	if store.hashes["goran@acme.io"] == "correct horse battery" {
		t.Fatal("password stored in plaintext")
	}
	if len(store.audits) != 1 || store.audits[0].Action != "auth.register" {
		t.Fatalf("audits = %+v", store.audits)
	}
}

func TestRegisterRejectsWeakPassword(t *testing.T) {
	accounts, store, _, _ := newTestAccounts(t)
	_, err := accounts.Register(context.Background(), RegisterInput{
		Email: "a@acme.io", Name: "A", Password: "short", OrgName: "Acme",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
	if len(store.users) != 0 {
		t.Fatal("user was created despite a weak password")
	}
}

func TestVerifyEmailConsumesToken(t *testing.T) {
	accounts, store, mail, _ := newTestAccounts(t)
	if _, err := accounts.Register(context.Background(), RegisterInput{
		Email: "a@acme.io", Name: "A", Password: "correct horse battery", OrgName: "Acme",
	}); err != nil {
		t.Fatal(err)
	}
	token := tokenFromLink(t, mail.verifications[0])

	if err := accounts.VerifyEmail(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if !store.users["a@acme.io"].EmailVerified {
		t.Fatal("user was not marked verified")
	}
	if err := accounts.VerifyEmail(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second verify err = %v, want ErrInvalidToken", err)
	}
}

func TestLogin(t *testing.T) {
	accounts, _, _, _ := newTestAccounts(t)
	if _, err := accounts.Register(context.Background(), RegisterInput{
		Email: "a@acme.io", Name: "A", Password: "correct horse battery", OrgName: "Acme",
	}); err != nil {
		t.Fatal(err)
	}

	pair, err := accounts.Login(context.Background(), LoginInput{
		Email: "A@Acme.io", Password: "correct horse battery",
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := accounts.tokens.issuer.Parse(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.OrgID == "" || claims.Role != "owner" {
		t.Fatalf("claims = %+v", claims)
	}

	if _, err := accounts.Login(context.Background(), LoginInput{Email: "a@acme.io", Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := accounts.Login(context.Background(), LoginInput{Email: "nobody@acme.io", Password: "whatever12345"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginWithoutOrganization(t *testing.T) {
	accounts, store, _, _ := newTestAccounts(t)
	if _, err := accounts.Register(context.Background(), RegisterInput{
		Email: "a@acme.io", Name: "A", Password: "correct horse battery", OrgName: "Acme",
	}); err != nil {
		t.Fatal(err)
	}
	store.memberships = map[string][2]string{}

	if _, err := accounts.Login(context.Background(), LoginInput{Email: "a@acme.io", Password: "correct horse battery"}); !errors.Is(err, ErrNoOrganization) {
		t.Fatalf("err = %v, want ErrNoOrganization", err)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	accounts, _, mail, sessions := newTestAccounts(t)
	if _, err := accounts.Register(context.Background(), RegisterInput{
		Email: "a@acme.io", Name: "A", Password: "correct horse battery", OrgName: "Acme",
	}); err != nil {
		t.Fatal(err)
	}
	pair, err := accounts.Login(context.Background(), LoginInput{Email: "a@acme.io", Password: "correct horse battery"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.tokens.Refresh(context.Background(), pair.RefreshToken, ""); err != nil {
		t.Fatal(err)
	}
	if sessions.active("user-1") != 1 {
		t.Fatalf("active sessions = %d, want 1", sessions.active("user-1"))
	}

	if err := accounts.RequestPasswordReset(context.Background(), "a@acme.io", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if len(mail.resets) != 1 {
		t.Fatalf("reset mails = %v", mail.resets)
	}

	token := tokenFromLink(t, mail.resets[0])
	if err := accounts.ResetPassword(context.Background(), token, "a new correct horse"); err != nil {
		t.Fatal(err)
	}
	if sessions.active("user-1") != 0 {
		t.Fatal("sessions were not revoked after password reset")
	}
	if _, err := accounts.Login(context.Background(), LoginInput{Email: "a@acme.io", Password: "a new correct horse"}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if err := accounts.ResetPassword(context.Background(), token, "another correct horse"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("reused reset token err = %v, want ErrInvalidToken", err)
	}
}

func TestRequestPasswordResetUnknownEmail(t *testing.T) {
	accounts, _, mail, _ := newTestAccounts(t)
	if err := accounts.RequestPasswordReset(context.Background(), "nobody@acme.io", "127.0.0.1"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(mail.resets) != 0 {
		t.Fatal("reset mail sent for an unknown account")
	}
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	_, token, ok := strings.Cut(link, "token=")
	if !ok || token == "" {
		t.Fatalf("link %q has no token", link)
	}
	return token
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0f]
	}
	return string(out)
}

func TestNewAccountsRequiresDependencies(t *testing.T) {
	if _, err := NewAccounts(nil, nil, nil, "", nil); err == nil {
		t.Fatal("expected error for missing dependencies")
	}
}

func TestRegisterDeduplicatesOrgSlug(t *testing.T) {
	fake := newFakeAccountStore()
	issuer, err := NewIssuer(testPEM(t), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := NewService(newMemStore(), issuer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := NewAccounts(fake, tokens, &fakeMailer{}, "https://app.test", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	for _, email := range []string{"a@acme.io", "b@acme.io"} {
		if _, err := accounts.Register(context.Background(), RegisterInput{
			Email: email, Name: "User", Password: "correct horse battery", OrgName: "Acme",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if !fake.orgSlugs["acme"] || !fake.orgSlugs["acme-2"] {
		t.Fatalf("slugs = %v, want acme and acme-2", fake.orgSlugs)
	}
}
