package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenIsSignedAndCached(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/42/access_tokens" {
			t.Errorf("path = %s", r.URL.Path)
		}
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.Split(auth, ".")
		if len(parts) != 3 {
			t.Fatalf("jwt parts = %d", len(parts))
		}
		var claims struct {
			Iss string `json:"iss"`
		}
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(raw, &claims)
		if claims.Iss != "123" {
			t.Fatalf("iss = %q", claims.Iss)
		}
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
			t.Fatalf("jwt signature: %v", err)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"ghs_test","expires_at":"` + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + `"}`))
	}))
	defer server.Close()

	client, err := New(123, 42, path, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first != "ghs_test" || second != "ghs_test" || calls != 1 {
		t.Fatalf("token=%q second=%q calls=%d", first, second, calls)
	}
}
