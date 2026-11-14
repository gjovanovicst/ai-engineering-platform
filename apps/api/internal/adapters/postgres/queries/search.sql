-- name: SearchTasks :many
SELECT * FROM tasks
WHERE org_id = $1
  AND ($2::uuid IS NULL OR project_id = $2)
  AND (
    title ILIKE '%' || $3 || '%'
    OR description ILIKE '%' || $3 || '%'
    OR ref ILIKE '%' || $3 || '%'
  )
ORDER BY updated_at DESC
LIMIT $4;
