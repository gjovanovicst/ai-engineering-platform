-- name: EnqueueMerge :one
INSERT INTO merge_queue (org_id, project_id, run_id, task_id, number, pull_request_id)
SELECT $1, $2, $3, $4, $5, $6
WHERE NOT EXISTS (
    SELECT 1 FROM merge_queue
    WHERE project_id = $2 AND number = $5 AND status IN ('queued','merging')
)
RETURNING id;

-- name: ClaimNextMerge :one
UPDATE merge_queue mq SET status = 'merging', updated_at = now()
WHERE mq.id = (
    SELECT m.id FROM merge_queue m
    WHERE m.org_id = $1 AND m.project_id = $2 AND m.status = 'queued'
    ORDER BY m.id LIMIT 1 FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteMerge :exec
UPDATE merge_queue SET status = $2, error = $3, updated_at = now() WHERE id = $1;

-- name: MergeQueuePosition :one
SELECT count(*)::bigint FROM merge_queue
WHERE project_id = $1 AND status = 'queued' AND id <= $2;

-- name: ListMergeQueue :many
SELECT * FROM merge_queue
WHERE org_id = $1 AND project_id = $2
ORDER BY id DESC LIMIT 50;
