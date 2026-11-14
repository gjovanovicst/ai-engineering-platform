-- name: ListOrgRepos :many
SELECT r.*, p.name AS project_name
FROM repos r JOIN projects p ON p.id = r.project_id
WHERE r.org_id = $1
ORDER BY r.created_at DESC LIMIT $2;

-- name: ListOrgRuns :many
SELECT r.*, COALESCE(t.ref, '') AS task_ref, COALESCE(t.title, '') AS task_title
FROM runs r LEFT JOIN tasks t ON t.id = r.task_id
WHERE r.org_id = $1
  AND ($2::text = '' OR r.status = $2)
  AND ($3::text = '' OR r.agent_type = $3)
ORDER BY r.created_at DESC LIMIT $4;

-- name: ListMembers :many
SELECT m.user_id, m.role, m.created_at, u.email, u.name
FROM memberships m JOIN users u ON u.id = m.user_id
WHERE m.org_id = $1
ORDER BY m.created_at ASC LIMIT $2;

-- name: ListAudit :many
SELECT id, actor_id, actor_kind, action, target, metadata, created_at, prev_hash, hash
FROM audit_log
WHERE org_id = $1
ORDER BY created_at DESC LIMIT $2;

-- name: UpdateMemberRole :one
UPDATE memberships SET role = $3 WHERE org_id = $1 AND user_id = $2 RETURNING role;

-- name: DeleteMember :exec
DELETE FROM memberships WHERE org_id = $1 AND user_id = $2;

-- name: RecordAudit :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target, metadata)
VALUES ($1, $2, 'human', $3, $4, $5);
