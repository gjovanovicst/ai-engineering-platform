// Command aip-kms-dev is a development-only KMS exposing POST /wrap and /unwrap.
// It is not a production KMS: no auth, no key isolation, no audit.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	key, err := parseKey(os.Getenv("AIP_KMS_DEV_KEY"))
	if err != nil {
		logger.Error("kms-dev key", slog.Any("err", err))
		os.Exit(1)
	}
	addr := os.Getenv("AIP_KMS_ADDR")
	if addr == "" {
		addr = ":18081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("POST /wrap", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Plaintext string `json:"plaintext"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		plain, err := base64.StdEncoding.DecodeString(in.Plaintext)
		if err != nil {
			http.Error(w, "bad plaintext", http.StatusBadRequest)
			return
		}
		sealed, err := seal(key, plain)
		if err != nil {
			http.Error(w, "seal", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"ciphertext": base64.StdEncoding.EncodeToString(sealed)})
	})
	mux.HandleFunc("POST /unwrap", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Ciphertext string `json:"ciphertext"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		sealed, err := base64.StdEncoding.DecodeString(in.Ciphertext)
		if err != nil {
			http.Error(w, "bad ciphertext", http.StatusBadRequest)
			return
		}
		plain, err := open(key, sealed)
		if err != nil {
			http.Error(w, "open", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"plaintext": base64.StdEncoding.EncodeToString(plain)})
	})
	logger.Warn("kms-dev is NOT a production KMS", slog.String("addr", addr))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("kms-dev", slog.Any("err", err))
		os.Exit(1)
	}
}

func parseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("AIP_KMS_DEV_KEY is required (32-byte hex/base64)")
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(raw)
	}
	if err != nil || len(key) != 32 {
		return nil, errors.New("AIP_KMS_DEV_KEY must be 32 bytes hex/base64")
	}
	return key, nil
}

func seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func open(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
}
