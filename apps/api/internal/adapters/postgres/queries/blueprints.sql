-- name: CreateBlueprint :one
INSERT INTO blueprints (org_id, name, version, spec)
VALUES ($1, $2, 1, $3)
RETURNING *;

-- name: ListBlueprints :many
SELECT * FROM blueprints WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: GetBlueprint :one
SELECT * FROM blueprints WHERE id = $1 AND org_id = $2;

-- name: PublishBlueprint :one
UPDATE blueprints
SET status = 'published', published_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'draft'
RETURNING *;

-- name: NextBlueprintVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next FROM blueprints WHERE org_id = $1 AND name = $2;

-- name: CreateBlueprintVersion :one
INSERT INTO blueprints (org_id, name, version, spec)
VALUES ($1, $2, $3, $4)
RETURNING *;
