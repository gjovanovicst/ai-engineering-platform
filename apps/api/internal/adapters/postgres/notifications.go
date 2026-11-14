package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gjovanovicst/ai-engineering-platform/internal/adapters/postgres/pgstore"
	"github.com/gjovanovicst/ai-engineering-platform/internal/app/notifications"
	"github.com/gjovanovicst/ai-engineering-platform/internal/platform/db"
)

// NotificationStore implements notifications.Store.
type NotificationStore struct {
	pool *pgxpool.Pool
	q    *pgstore.Queries
}

// NewNotificationStore wires the store.
func NewNotificationStore(pool *pgxpool.Pool) *NotificationStore {
	return &NotificationStore{pool: pool, q: pgstore.New(pool)}
}

// Insert stores a notification.
func (s *NotificationStore) Insert(ctx context.Context, orgID, userID, kind, title, body, link string) (notifications.Notification, error) {
	var out notifications.Notification
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := s.q.WithTx(tx).InsertNotification(ctx, pgstore.InsertNotificationParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Type: kind, Title: title, Body: body, Link: link,
		})
		if err != nil {
			return err
		}
		out = notificationFromRow(row)
		return nil
	})
	return out, err
}

// List returns the user's notifications.
func (s *NotificationStore) List(ctx context.Context, orgID, userID string, unreadOnly bool, limit int) ([]notifications.Notification, error) {
	var out []notifications.Notification
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListNotifications(ctx, pgstore.ListNotificationsParams{
			OrgID: toUUID(orgID), UserID: toUUID(userID), Column3: unreadOnly, Limit: int32(limit), //nolint:gosec // clamped
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, notificationFromRow(row))
		}
		return nil
	})
	return out, err
}

// MarkRead marks the given notifications read (empty = all).
func (s *NotificationStore) MarkRead(ctx context.Context, orgID, userID string, ids []string) error {
	return db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if len(ids) == 0 {
			return s.q.WithTx(tx).MarkAllNotificationsRead(ctx, pgstore.MarkAllNotificationsReadParams{
				OrgID: toUUID(orgID), UserID: toUUID(userID),
			})
		}
		for _, id := range ids {
			if err := s.q.WithTx(tx).MarkNotificationRead(ctx, pgstore.MarkNotificationReadParams{
				ID: toUUID(id), OrgID: toUUID(orgID), UserID: toUUID(userID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Admins lists org owners/admins.
func (s *NotificationStore) Admins(ctx context.Context, orgID string) ([]notifications.Admin, error) {
	var out []notifications.Admin
	err := db.WithOrg(ctx, s.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := s.q.WithTx(tx).ListOrgAdmins(ctx, toUUID(orgID))
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, notifications.Admin{UserID: uuidString(row.UserID), Email: row.Email})
		}
		return nil
	})
	return out, err
}

func notificationFromRow(row pgstore.Notification) notifications.Notification {
	return notifications.Notification{
		ID: uuidString(row.ID), Type: row.Type, Title: row.Title, Body: row.Body,
		Link: row.Link, Read: row.ReadAt.Valid, CreatedAt: row.CreatedAt.Time,
	}
}

var _ notifications.Store = (*NotificationStore)(nil)
