-- name: CreateProvider :one
INSERT INTO ai_provider_configs (org_id, name, base_url, model, api_key_enc, key_hint, kind)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListProviders :many
SELECT * FROM ai_provider_configs WHERE org_id = $1 ORDER BY created_at ASC;

-- name: GetProvider :one
SELECT * FROM ai_provider_configs WHERE id = $1 AND org_id = $2;

-- name: DefaultProvider :one
SELECT * FROM ai_provider_configs WHERE org_id = $1 AND enabled = true ORDER BY created_at ASC LIMIT 1;

-- name: RotateProviderKey :one
UPDATE ai_provider_configs
SET api_key_enc = $3, key_hint = $4
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProviderEnabled :one
UPDATE ai_provider_configs
SET enabled = $3
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteProvider :exec
DELETE FROM ai_provider_configs WHERE id = $1 AND org_id = $2;
