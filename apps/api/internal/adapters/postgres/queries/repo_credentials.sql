-- name: UpsertRepoCredential :one
-- created_by resolves to NULL when the caller has no users row (for example an
-- API key whose subject is "api-key:<id>"), so the write still succeeds.
INSERT INTO repo_credentials (org_id, repo_id, ciphertext, hint, created_by)
VALUES (
    sqlc.arg(org_id), sqlc.arg(repo_id), sqlc.arg(ciphertext), sqlc.arg(hint),
    (SELECT u.id FROM users u WHERE u.id = sqlc.arg(actor_id))
)
ON CONFLICT (repo_id) DO UPDATE
SET ciphertext = EXCLUDED.ciphertext,
    hint = EXCLUDED.hint,
    created_by = EXCLUDED.created_by,
    updated_at = now()
RETURNING *;

-- name: GetRepoCredential :one
SELECT * FROM repo_credentials WHERE org_id = $1 AND repo_id = $2;

-- name: DeleteRepoCredential :execrows
DELETE FROM repo_credentials WHERE org_id = $1 AND repo_id = $2;

-- name: RecordRepoCredentialAudit :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target, metadata)
VALUES (
    sqlc.arg(org_id),
    (SELECT u.id FROM users u WHERE u.id = sqlc.arg(actor_id)),
    'user', sqlc.arg(action), sqlc.arg(target), sqlc.arg(metadata)
);
