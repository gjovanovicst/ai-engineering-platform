-- name: CreateSession :one
INSERT INTO sessions (user_id, org_id, role, refresh_hash, platform, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: SessionByRefreshHash :one
SELECT * FROM sessions WHERE refresh_hash = $1;

-- name: RotateSession :exec
UPDATE sessions
SET revoked_at = $2, revoked_reason = $3
WHERE id = $1;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = $2, revoked_reason = $3
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllForUser :exec
UPDATE sessions
SET revoked_at = $2, revoked_reason = $3
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions SET last_used_at = $2 WHERE id = $1;
