package kms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapUnwrap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in request
		_ = json.NewDecoder(r.Body).Decode(&in)
		switch r.URL.Path {
		case "/wrap":
			_ = json.NewEncoder(w).Encode(response{Ciphertext: base64.StdEncoding.EncodeToString([]byte("wrapped:" + in.Plaintext))})
		case "/unwrap":
			plain, _ := base64.StdEncoding.DecodeString(in.Ciphertext)
			_ = json.NewEncoder(w).Encode(response{Plaintext: base64.StdEncoding.EncodeToString([]byte("plain:" + string(plain)))})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client := New(srv.URL, "tok")
	wrapped, err := client.Wrap(context.Background(), []byte("dek"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := client.Unwrap(context.Background(), wrapped)
	if err != nil || string(plain) != "plain:wrapped:ZGVr" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
}
