-- name: CreateTaskAttachment :one
INSERT INTO task_attachments (org_id, task_id, filename, content_type, size_bytes, sha256, data, uploaded_by, object_key)
SELECT $1, $2, $3, $4, $5, $6, $7, $8, $10
WHERE EXISTS (SELECT 1 FROM tasks WHERE id = $2 AND org_id = $1 AND project_id = $9)
RETURNING id, org_id, task_id, filename, content_type, size_bytes, sha256, uploaded_by, created_at;

-- name: ListTaskAttachments :many
SELECT id, org_id, task_id, filename, content_type, size_bytes, sha256, uploaded_by, created_at
FROM task_attachments
WHERE org_id = $1 AND task_id = $2
ORDER BY created_at;

-- name: GetTaskAttachment :one
SELECT id, org_id, task_id, filename, content_type, size_bytes, sha256, uploaded_by, created_at, data, object_key
FROM task_attachments
WHERE id = $1 AND org_id = $2 AND task_id = $3;
