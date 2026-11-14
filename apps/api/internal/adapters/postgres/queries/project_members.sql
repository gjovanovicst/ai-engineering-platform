-- name: UpsertProjectMember :one
INSERT INTO project_members (org_id, project_id, user_id, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role
RETURNING user_id, role, created_at;

-- name: ListProjectMembers :many
SELECT user_id, role, created_at FROM project_members
WHERE org_id = $1 AND project_id = $2 ORDER BY created_at;

-- name: DeleteProjectMember :exec
DELETE FROM project_members WHERE org_id = $1 AND project_id = $2 AND user_id = $3;

-- name: ProjectAccess :one
SELECT COALESCE((SELECT pm.role FROM project_members pm WHERE pm.org_id = $1 AND pm.project_id = $2 AND pm.user_id = $3), '')::text AS role,
       (SELECT count(*)::bigint FROM project_members pm2 WHERE pm2.org_id = $1 AND pm2.project_id = $2) AS members
;
