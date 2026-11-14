-- name: UpsertRunArtifact :one
INSERT INTO run_artifacts (org_id, run_id, kind, bucket_key, sha256, bytes)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (run_id, kind)
DO UPDATE SET bucket_key = EXCLUDED.bucket_key, sha256 = EXCLUDED.sha256, bytes = EXCLUDED.bytes
RETURNING *;

-- name: ListRunArtifacts :many
SELECT * FROM run_artifacts WHERE org_id = $1 AND run_id = $2 ORDER BY created_at;
