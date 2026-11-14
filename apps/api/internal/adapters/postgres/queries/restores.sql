-- name: CreateRestoreRequest :one
INSERT INTO restore_requests (org_id, target_time, requested_by)
VALUES ($1, $2, $3)
RETURNING id, target_time, status, detail, created_at;

-- name: ListRestoreRequests :many
SELECT id, target_time, status, detail, created_at
FROM restore_requests WHERE org_id = $1 ORDER BY created_at DESC LIMIT 50;
