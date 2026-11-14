-- name: ListProjectComponents :many
SELECT c.id, c.name, c.category, c.status, c.import_path, c.package, c.version,
       pc.mode, pc.reason
FROM project_components pc
JOIN component_entries c ON c.id = pc.component_id
WHERE pc.project_id = $1 AND pc.org_id = $2
ORDER BY c.name;

-- name: DeleteProjectComponents :exec
DELETE FROM project_components WHERE project_id = $1 AND org_id = $2;

-- name: InsertProjectComponent :execrows
INSERT INTO project_components (org_id, project_id, component_id, mode, reason, created_by)
SELECT $1, $2, c.id, $5, $6, $4
FROM component_entries c
WHERE c.id = $3 AND c.org_id = $1 AND c.status IN ('approved','published')
ON CONFLICT (project_id, component_id) DO UPDATE
SET mode = EXCLUDED.mode, reason = EXCLUDED.reason;

-- name: ListForbiddenComponents :many
SELECT name, import_path FROM component_entries
WHERE org_id = $1 AND status IN ('forbidden','deprecated') AND import_path <> ''
ORDER BY name;
