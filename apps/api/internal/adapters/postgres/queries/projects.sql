-- name: CreateProject :one
INSERT INTO projects (org_id, name, slug, description, status, blueprint_id)
VALUES ($1, $2, $3, $4, 'active', $5)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1 AND org_id = $2 AND archived_at IS NULL;

-- name: ListProjects :many
SELECT * FROM projects
WHERE org_id = $1 AND archived_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateProjectRequiredChecks :one
UPDATE projects SET required_checks = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectE2ECommand :one
UPDATE projects SET e2e_command = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectVisualCommand :one
UPDATE projects SET visual_command = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectMCP :one
UPDATE projects SET mcp_servers = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectVisualPolicy :one
UPDATE projects SET visual_threshold = $3, visual_masks = $4, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectPolicy :one
UPDATE projects SET policy_max_diff_bytes = $3, policy_forbidden_paths = COALESCE($4::text[], '{}'), updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectChangeWindow :one
UPDATE projects SET policy_change_window = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectAllowedModels :one
UPDATE projects SET policy_allowed_models = COALESCE($3::text[], '{}'), updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectAllowedTools :one
UPDATE projects SET policy_allowed_tools = COALESCE($3::text[], '{}'), updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectRequiredApprovals :one
UPDATE projects SET policy_required_approvals = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: SetProjectPluginChecks :one
UPDATE projects SET plugin_checks = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1 AND org_id = $2;
