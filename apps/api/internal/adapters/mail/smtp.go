// Package mail sends transactional email through SMTP (Mailhog in development,
// any SMTP relay in production). It never logs message bodies.
package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTP sends plain-text transactional mail.
type SMTP struct {
	addr string
	from string
}

// NewSMTP returns a sender for the given host:port and From address.
func NewSMTP(addr, from string) *SMTP {
	return &SMTP{addr: addr, from: from}
}

// SendVerification sends the email verification link.
func (s *SMTP) SendVerification(ctx context.Context, to, link string) error {
	return s.send(ctx, to, "Verify your email",
		"Confirm your account by opening this link:\n\n"+link+"\n\nIt expires in 24 hours.")
}

// SendInvite sends the invitation link.
func (s *SMTP) SendInvite(ctx context.Context, to, link string) error {
	return s.send(ctx, to, "You were invited to an AI Engineering Platform workspace",
		"Accept the invitation:\n\n"+link+"\n\nThe link expires in 7 days.")
}

// SendPasswordReset sends the password reset link.
func (s *SMTP) SendPasswordReset(ctx context.Context, to, link string) error {
	return s.send(ctx, to, "Reset your password",
		"Use this link to choose a new password:\n\n"+link+"\n\nIt expires in 1 hour.")
}

// SendNotification sends an in-app notification by email.
func (s *SMTP) SendNotification(ctx context.Context, to, title, body, link string) error {
	text := body
	if link != "" {
		text += "\n\nOpen: " + link
	}
	return s.send(ctx, to, title, text)
}

func (s *SMTP) send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	msg := strings.Join([]string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	if err := smtp.SendMail(s.addr, nil, s.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("mail: send to %s: %w", to, err)
	}
	return nil
}
