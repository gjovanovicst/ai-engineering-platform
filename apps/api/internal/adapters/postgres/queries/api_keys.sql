-- name: CreateApiKey :one
INSERT INTO api_keys (org_id, name, key_hash, scopes, role, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, role, created_at, expires_at;

-- name: ApiKeyByHash :one
SELECT id, org_id, name, role, scopes, revoked_at, created_at, expires_at
FROM api_keys WHERE org_id = $1 AND key_hash = $2;

-- name: ListApiKeys :many
SELECT id, name, role, scopes, last_used_at, revoked_at, created_at, expires_at
FROM api_keys WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100;

-- name: RevokeApiKey :exec
UPDATE api_keys SET revoked_at = now() WHERE org_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: TouchApiKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;
