-- name: CreateDesignStandard :one
INSERT INTO design_standards (org_id, name, version, spec, sha256)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: NextDesignStandardVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next
FROM design_standards WHERE org_id = $1 AND name = $2;

-- name: ListDesignStandards :many
SELECT * FROM design_standards WHERE org_id = $1
ORDER BY name ASC, version DESC LIMIT $2;

-- name: GetDesignStandard :one
SELECT * FROM design_standards WHERE id = $1 AND org_id = $2;

-- name: PublishDesignStandard :one
UPDATE design_standards SET status = 'published', published_at = now(), updated_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'draft'
RETURNING *;

-- name: RecordDesignAudit :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target, metadata)
VALUES ($1, $2, 'human', $3, $4, $5);
