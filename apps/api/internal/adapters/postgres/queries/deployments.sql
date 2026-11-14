-- name: CreateEnvironment :one
INSERT INTO environments (org_id, project_id, name, url)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEnvironments :many
SELECT * FROM environments WHERE org_id = $1 AND project_id = $2 ORDER BY name;

-- name: GetEnvironment :one
SELECT * FROM environments WHERE id = $1 AND org_id = $2;

-- name: CreateDeployment :one
INSERT INTO deployments (org_id, environment_id, release_ref, status, deployed_by, started_at, finished_at, log)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListDeployments :many
SELECT * FROM deployments WHERE org_id = $1 AND environment_id = $2 ORDER BY started_at DESC LIMIT $3;

-- name: GetDeployment :one
SELECT * FROM deployments WHERE id = $1 AND org_id = $2;

-- name: ApproveDeployment :one
UPDATE deployments
SET status = 'succeeded', approved_by = $3, approved_at = now(), finished_at = now(),
    log = log || 'approved by ' || $4 || E'\n'
WHERE id = $1 AND org_id = $2 AND status = 'queued'
RETURNING *;
