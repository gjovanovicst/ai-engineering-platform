// Package notifications stores in-app notifications per user.
package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Notification is one in-app message.
type Notification struct {
	ID        string
	Type      string
	Title     string
	Body      string
	Link      string
	Read      bool
	CreatedAt time.Time
}

// Store persists notifications.
type Store interface {
	Insert(ctx context.Context, orgID, userID, kind, title, body, link string) (Notification, error)
	List(ctx context.Context, orgID, userID string, unreadOnly bool, limit int) ([]Notification, error)
	MarkRead(ctx context.Context, orgID, userID string, ids []string) error
	Admins(ctx context.Context, orgID string) ([]Admin, error)
}

// Admin is a recipient for org-wide events.
type Admin struct {
	UserID string
	Email  string
}

// Emailer delivers a notification by email.
type Emailer interface {
	SendNotification(ctx context.Context, to, title, body, link string) error
}

// Service validates and fans out notifications.
type Service struct {
	store   Store
	emailer Emailer
	baseURL string
	log     *slog.Logger
}

// New wires the service.
func New(store Store, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{store: store, log: log}
}

// WithEmailer enables email delivery; links are made absolute with baseURL.
func (s *Service) WithEmailer(emailer Emailer, baseURL string) *Service {
	s.emailer = emailer
	s.baseURL = baseURL
	return s
}

// Notify stores one notification.
func (s *Service) Notify(ctx context.Context, orgID, userID, kind, title, body, link string) error {
	if userID == "" || title == "" {
		return fmt.Errorf("%w: user and title are required", domain.ErrValidation)
	}
	_, err := s.store.Insert(ctx, orgID, userID, kind, title, body, link)
	return err
}

// NotifyAdmins sends the same notification to every owner/admin of the org.
func (s *Service) NotifyAdmins(ctx context.Context, orgID, kind, title, body, link string) error {
	admins, err := s.store.Admins(ctx, orgID)
	if err != nil {
		return err
	}
	for _, admin := range admins {
		if _, err := s.store.Insert(ctx, orgID, admin.UserID, kind, title, body, link); err != nil {
			s.log.Warn("notifications: insert", slog.String("user", admin.UserID), slog.Any("err", err))
			continue
		}
		if s.emailer != nil && admin.Email != "" {
			target := link
			if target != "" && s.baseURL != "" && !strings.HasPrefix(target, "http") {
				target = strings.TrimRight(s.baseURL, "/") + target
			}
			if err := s.emailer.SendNotification(ctx, admin.Email, title, body, target); err != nil {
				s.log.Warn("notifications: email", slog.String("user", admin.UserID), slog.Any("err", err))
			}
		}
	}
	return nil
}

// List returns the user's notifications (optionally only unread).
func (s *Service) List(ctx context.Context, orgID, userID string, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.List(ctx, orgID, userID, unreadOnly, limit)
}

// MarkRead marks the given notifications read; an empty list means all.
func (s *Service) MarkRead(ctx context.Context, orgID, userID string, ids []string) error {
	return s.store.MarkRead(ctx, orgID, userID, ids)
}
