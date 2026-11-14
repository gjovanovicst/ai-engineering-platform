-- name: GetOrg :one
SELECT id, slug, name, max_concurrent_runs, legal_hold, region, created_at, updated_at FROM orgs WHERE id = $1;

-- name: UpdateOrgName :one
UPDATE orgs SET name = $2, updated_at = now() WHERE id = $1
RETURNING id, slug, name, max_concurrent_runs, legal_hold, region, created_at, updated_at;

-- name: DeleteOrg :exec
DELETE FROM orgs WHERE id = $1;

-- name: RecordOrgAudit :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target, metadata)
VALUES ($1, $2, 'human', $3, $4, $5);

-- name: SetOrgLegalHold :one
UPDATE orgs SET legal_hold = $2, updated_at = now() WHERE id = $1
RETURNING id, slug, name, max_concurrent_runs, legal_hold, region, created_at, updated_at;

-- name: SetOrgRegion :one
UPDATE orgs SET region = $2, updated_at = now() WHERE id = $1
RETURNING id, slug, name, max_concurrent_runs, legal_hold, region, created_at, updated_at;
