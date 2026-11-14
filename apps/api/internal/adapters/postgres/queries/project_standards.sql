-- name: SetProjectDesignStandard :one
UPDATE projects p
SET design_system_id = ds.id, design_standard_version = ds.version, updated_at = now()
FROM design_standards ds
WHERE p.id = $1 AND p.org_id = $2 AND ds.id = $3 AND ds.org_id = $2 AND ds.status = 'published'
RETURNING p.*;

-- name: ClearProjectDesignStandard :one
UPDATE projects SET design_system_id = NULL, design_standard_version = NULL, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: GetProjectDesignStandard :one
SELECT ds.id, ds.name, ds.version, ds.status, ds.sha256, ds.published_at
FROM projects p
JOIN design_standards ds ON ds.id = p.design_system_id
WHERE p.id = $1 AND p.org_id = $2;
