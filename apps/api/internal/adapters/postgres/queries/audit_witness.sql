-- name: AuditHead :one
SELECT count(*)::bigint AS rows,
       COALESCE(max(id), 0)::bigint AS last_id,
       COALESCE((SELECT a.hash FROM audit_log a WHERE a.org_id = $1 ORDER BY a.id DESC LIMIT 1), '')::text AS last_hash
FROM audit_log WHERE org_id = $1;

-- name: AuditHashAt :one
SELECT hash FROM audit_log WHERE org_id = $1 AND id = $2;

-- name: InsertAuditWitness :one
INSERT INTO audit_witnesses (org_id, object_key, row_count, last_id, last_hash)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, object_key, row_count, last_id, last_hash, created_at;

-- name: LatestAuditWitness :one
SELECT id, object_key, row_count, last_id, last_hash, created_at
FROM audit_witnesses WHERE org_id = $1 ORDER BY created_at DESC LIMIT 1;
