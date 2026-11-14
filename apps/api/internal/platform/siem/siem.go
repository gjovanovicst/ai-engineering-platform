// Package siem forwards audit notifications to a SIEM webhook.
package siem

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sender posts audit events.
type Sender struct {
	URL    string
	Secret []byte
	Client *http.Client
}

// Send POSTs one payload with an optional HMAC signature.
func (s *Sender) Send(ctx context.Context, payload []byte) error {
	if s.URL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if len(s.Secret) > 0 {
		mac := hmac.New(sha256.New, s.Secret)
		mac.Write(payload)
		req.Header.Set("X-AIP-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("siem: post: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("siem: status %d: %s", res.StatusCode, string(body))
	}
	return nil
}

// Run listens for audit notifications until ctx ends.
func Run(ctx context.Context, pool *pgxpool.Pool, sender *Sender, log *slog.Logger) {
	if sender == nil || sender.URL == "" {
		return
	}
	for ctx.Err() == nil {
		if err := listen(ctx, pool, sender, log); err != nil && ctx.Err() == nil {
			log.Warn("siem: listener", slog.Any("err", err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func listen(ctx context.Context, pool *pgxpool.Pool, sender *Sender, log *slog.Logger) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN aip_audit"); err != nil {
		return err
	}
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if err := sender.Send(ctx, []byte(notification.Payload)); err != nil {
			log.Warn("siem: forward", slog.Any("err", err))
		}
	}
}
