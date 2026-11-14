-- name: InsertNotification :one
INSERT INTO notifications (org_id, user_id, type, title, body, link)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
WHERE org_id = $1 AND user_id = $2 AND ($3::bool = false OR read_at IS NULL)
ORDER BY created_at DESC LIMIT $4;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE org_id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read_at = now() WHERE id = $1 AND org_id = $2 AND user_id = $3 AND read_at IS NULL;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = now() WHERE org_id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: ListOrgAdmins :many
SELECT u.id AS user_id, u.email
FROM memberships m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1 AND m.role IN ('owner','admin');
