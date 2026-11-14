-- name: CreatePreview :one
INSERT INTO preview_environments (org_id, project_id, name, image, port, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListPreviews :many
SELECT * FROM preview_environments
WHERE org_id = $1 AND project_id = $2
ORDER BY created_at DESC LIMIT 50;

-- name: GetPreview :one
SELECT * FROM preview_environments WHERE org_id = $1 AND id = $2;

-- name: SetPreviewStatus :exec
UPDATE preview_environments
SET status = $3, url = $4, container_id = $5, error = $6, updated_at = now()
WHERE org_id = $1 AND id = $2;

-- name: RequestedPreviews :many
SELECT * FROM preview_environments
WHERE status IN ('requested','destroying') AND org_id = $1
ORDER BY created_at LIMIT $2;
