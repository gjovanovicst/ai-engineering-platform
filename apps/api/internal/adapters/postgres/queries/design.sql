-- name: ListDesignTokens :many
SELECT * FROM design_tokens WHERE org_id = $1 ORDER BY category, name;

-- name: CreateDesignToken :one
INSERT INTO design_tokens (org_id, name, value, category)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateDesignToken :one
UPDATE design_tokens SET value = $3, category = $4, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteDesignToken :exec
DELETE FROM design_tokens WHERE id = $1 AND org_id = $2;

-- name: CountDesignTokens :one
SELECT count(*) FROM design_tokens WHERE org_id = $1;

-- name: ListComponents :many
SELECT * FROM component_entries WHERE org_id = $1 ORDER BY name;

-- name: CreateComponent :one
INSERT INTO component_entries (org_id, name, purpose, rules, status, category, framework, package, import_path, version, docs, example)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateComponent :one
UPDATE component_entries
SET purpose = $3, rules = $4, status = $5, category = $6, framework = $7,
    package = $8, import_path = $9, version = $10, docs = $11, example = $12, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteComponent :exec
DELETE FROM component_entries WHERE id = $1 AND org_id = $2;

-- name: CountComponents :one
SELECT count(*) FROM component_entries WHERE org_id = $1;
