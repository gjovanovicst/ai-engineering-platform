package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"
)

type memStore struct {
	mu     sync.Mutex
	byHash map[string]*Session
	byID   map[string]*Session
}

func newMemStore() *memStore {
	return &memStore{byHash: map[string]*Session{}, byID: map[string]*Session{}}
}

func (m *memStore) Create(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put(s)
	return nil
}

func (m *memStore) put(s Session) {
	copy := s
	m.byHash[hex.EncodeToString(s.RefreshHash)] = &copy
	m.byID[s.ID] = &copy
}

func (m *memStore) ByRefreshHash(_ context.Context, hash []byte) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byHash[hex.EncodeToString(hash)]
	if !ok {
		return Session{}, errors.New("not found")
	}
	return *s, nil
}

func (m *memStore) Rotate(_ context.Context, oldID string, next Session, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeLocked(oldID, "rotated", at)
	m.put(next)
	return nil
}

func (m *memStore) RevokeSession(_ context.Context, id, _ string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeLocked(id, "revoked", at)
	return nil
}

func (m *memStore) RevokeAllForUser(_ context.Context, userID, _ string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.byID {
		if s.UserID == userID && s.RevokedAt == nil {
			t := at
			s.RevokedAt = &t
		}
	}
	return nil
}

func (m *memStore) Touch(_ context.Context, _ string, _ time.Time) error { return nil }

func (m *memStore) revokeLocked(id, _ string, at time.Time) {
	if s, ok := m.byID[id]; ok && s.RevokedAt == nil {
		t := at
		s.RevokedAt = &t
	}
}

func (m *memStore) active(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.byID {
		if s.UserID == userID && s.RevokedAt == nil {
			n++
		}
	}
	return n
}

func (m *memStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.byID)
}

func newTestService(t *testing.T) (*Service, *memStore) {
	t.Helper()
	issuer, err := NewIssuer(testPEM(t), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	svc, err := NewService(store, issuer, 720*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store
}

func TestIssueCreatesSession(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	pair, err := svc.Issue(ctx, IssueRequest{UserID: "user-1", OrgID: "org-1", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("empty token pair")
	}
	if store.count() != 1 {
		t.Fatalf("sessions = %d, want 1", store.count())
	}

	claims, err := svc.issuer.Parse(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.OrgID != "org-1" || claims.Role != "admin" || claims.SessionID == "" {
		t.Fatalf("claims = %+v", claims)
	}

	session, err := store.ByRefreshHash(ctx, hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	if session.Platform != PlatformWeb {
		t.Fatalf("platform = %q, want web", session.Platform)
	}
	if string(session.RefreshHash) == pair.RefreshToken {
		t.Fatal("refresh token stored in plaintext")
	}
}

func TestRefreshRotatesAndPreservesClaims(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	first, err := svc.Issue(ctx, IssueRequest{UserID: "user-1", OrgID: "org-1", Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Refresh(ctx, first.RefreshToken, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if store.count() != 2 {
		t.Fatalf("sessions = %d, want 2", store.count())
	}
	if store.active("user-1") != 1 {
		t.Fatalf("active sessions = %d, want 1", store.active("user-1"))
	}

	claims, err := svc.issuer.Parse(second.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.OrgID != "org-1" || claims.Role != "member" {
		t.Fatalf("claims after refresh = %+v", claims)
	}
}

func TestRefreshReuseRevokesAllSessions(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	first, err := svc.Issue(ctx, IssueRequest{UserID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, first.RefreshToken, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, first.RefreshToken, ""); !errors.Is(err, ErrRefreshReuse) {
		t.Fatalf("err = %v, want ErrRefreshReuse", err)
	}
	if store.active("user-1") != 0 {
		t.Fatalf("active sessions = %d, want 0 after reuse", store.active("user-1"))
	}
}

func TestRefreshExpiredSession(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	first, err := svc.Issue(ctx, IssueRequest{UserID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.ByRefreshHash(ctx, hashToken(first.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	session.ExpiresAt = time.Now().Add(-time.Hour)
	store.put(session)

	if _, err := svc.Refresh(ctx, first.RefreshToken, ""); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Refresh(context.Background(), "nope", ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestIssueRejectsInvalidPlatform(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Issue(context.Background(), IssueRequest{UserID: "u", Platform: "toaster"}); !errors.Is(err, ErrInvalidPlatform) {
		t.Fatalf("err = %v, want ErrInvalidPlatform", err)
	}
}

func TestRevokeEndsSession(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()

	pair, err := svc.Issue(ctx, IssueRequest{UserID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.ByRefreshHash(ctx, hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, session.ID, "logout"); err != nil {
		t.Fatal(err)
	}
	if store.active("user-1") != 0 {
		t.Fatal("session still active after revoke")
	}
}

func TestIdleTimeoutEndsSession(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	pair, err := svc.Issue(ctx, IssueRequest{UserID: "user-1", OrgID: "org-1", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.ByRefreshHash(ctx, hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	store.mu.Lock()
	store.byID[sess.ID].LastUsedAt = &old
	store.byHash[hex.EncodeToString(hashToken(pair.RefreshToken))].LastUsedAt = &old
	store.mu.Unlock()

	svc.WithIdleTTL(time.Hour)
	if _, err := svc.Refresh(ctx, pair.RefreshToken, PlatformWeb); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}
