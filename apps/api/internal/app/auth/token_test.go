package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"
)

func testPEM(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func TestIssueAndParse(t *testing.T) {
	issuer, err := NewIssuer(testPEM(t), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw, expires, err := issuer.Issue("user-1", Claims{OrgID: "org-1", Role: "admin", SessionID: "sess-1"})
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(expires) <= 0 {
		t.Fatal("expiry is in the past")
	}

	claims, err := issuer.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.OrgID != "org-1" || claims.Role != "admin" || claims.SessionID != "sess-1" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestParseRejectsTamperedToken(t *testing.T) {
	issuer, err := NewIssuer(testPEM(t), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := issuer.Issue("user-1", Claims{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	// corrupt the first byte of the signature segment
	parts := strings.Split(raw, ".")
	sig := parts[2]
	replacement := "A"
	if strings.HasPrefix(sig, "A") {
		replacement = "B"
	}
	parts[2] = replacement + sig[1:]
	tampered := strings.Join(parts, ".")
	if _, err := issuer.Parse(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	issuer, err := NewIssuer(testPEM(t), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	issuer.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	raw, _, err := issuer.Issue("user-1", Claims{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestRejectsNonEd25519Key(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("nope")})
	if _, err := NewIssuer(block, time.Minute); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestPublicKeyPEM(t *testing.T) {
	issuer, err := NewIssuer(testPEM(t), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := issuer.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "PUBLIC KEY" {
		t.Fatalf("public key PEM = %q", raw)
	}
}
