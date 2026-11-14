-- name: CreateIncident :one
INSERT INTO incidents (org_id, project_id, title, severity, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, title, severity, status, opened_at, resolved_at;

-- name: ListIncidents :many
SELECT id, project_id, title, severity, status, opened_at, resolved_at
FROM incidents WHERE org_id = $1 ORDER BY opened_at DESC LIMIT $2;

-- name: ResolveIncident :one
UPDATE incidents SET status = 'resolved', resolved_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'open'
RETURNING id, project_id, title, severity, status, opened_at, resolved_at;
