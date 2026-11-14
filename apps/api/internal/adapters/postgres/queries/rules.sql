-- name: CreateRule :one
INSERT INTO rules (org_id, project_id, level, scope_ref, name)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRules :many
SELECT * FROM rules WHERE org_id = $1 AND ($2::uuid IS NULL OR project_id = $2)
ORDER BY created_at ASC LIMIT $3;

-- name: GetRule :one
SELECT * FROM rules WHERE id = $1 AND org_id = $2;

-- name: CreateRuleVersion :one
INSERT INTO rule_versions (rule_id, version, body, sha256)
SELECT $1, COALESCE(MAX(version), 0) + 1, $2, $3 FROM rule_versions WHERE rule_id = $1
RETURNING *;

-- name: ListRuleVersions :many
SELECT * FROM rule_versions WHERE rule_id = $1 ORDER BY version DESC;

-- name: PublishRuleVersion :one
UPDATE rule_versions SET published_at = now(), published_by = $3
WHERE rule_id = $1 AND version = $2 AND published_at IS NULL
RETURNING *;

-- name: ActiveRules :many
SELECT r.id, r.name, r.level, r.project_id, v.version, v.body, v.sha256
FROM rules r
JOIN rule_versions v ON v.rule_id = r.id
WHERE r.org_id = $1
  AND (r.project_id IS NULL OR r.project_id = $2)
  AND v.version = (
    SELECT max(v2.version) FROM rule_versions v2
    WHERE v2.rule_id = r.id AND v2.published_at IS NOT NULL
  )
ORDER BY r.level, r.name;
