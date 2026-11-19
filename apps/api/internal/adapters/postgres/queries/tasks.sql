-- name: CreateTask :one
INSERT INTO tasks (org_id, project_id, ref, title, description, acceptance, priority, tags, created_by, depends_on)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL;

-- name: GetTaskByRef :one
SELECT * FROM tasks
WHERE org_id = $1 AND project_id = $2 AND ref = $3 AND deleted_at IS NULL;

-- name: CountTasks :one
SELECT count(*) FROM tasks WHERE project_id = $1;

-- name: ListTasks :many
SELECT * FROM tasks
WHERE org_id = $1
  AND project_id = $2
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('assignee')::uuid IS NULL OR assignee_id = sqlc.narg('assignee')::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: UpdateTaskStatus :one
UPDATE tasks
SET status = $3,
    blocked_reason = $4,
    ready_at = $5,
    done_at = $6
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateTask :one
UPDATE tasks
SET title = $3, description = $4, priority = $5, assignee_id = $6, tags = $7,
    acceptance = $8, depends_on = $9
WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: NextTaskRef :one
UPDATE projects SET task_seq = task_seq + 1, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING task_seq;
