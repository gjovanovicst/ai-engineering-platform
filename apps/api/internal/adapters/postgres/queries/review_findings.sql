-- name: ListReviewFindings :many
SELECT id, org_id, project_id, task_id, run_id, severity, file, line, message,
       status, decided_by, reason, fixed_by_run, created_at, updated_at
FROM review_findings
WHERE org_id = $1 AND task_id = $2
ORDER BY created_at, id;

-- name: InsertReviewFinding :one
INSERT INTO review_findings (org_id, project_id, task_id, run_id, severity, file, line, message)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, org_id, project_id, task_id, run_id, severity, file, line, message,
          status, decided_by, reason, fixed_by_run, created_at, updated_at;

-- name: TouchReviewFinding :exec
UPDATE review_findings
SET run_id = $3, severity = $4, updated_at = now()
WHERE org_id = $1 AND id = $2;

-- name: MarkReviewFindingFixed :exec
UPDATE review_findings
SET status = 'fixed', fixed_by_run = $3, updated_at = now()
WHERE org_id = $1 AND id = $2 AND status = 'open';

-- name: DecideReviewFinding :one
UPDATE review_findings
SET status = $3, decided_by = $4, reason = $5, updated_at = now()
WHERE org_id = $1 AND id = $2
RETURNING id, org_id, project_id, task_id, run_id, severity, file, line, message,
          status, decided_by, reason, fixed_by_run, created_at, updated_at;

-- name: CountOpenReviewBlockers :one
SELECT count(*) FROM review_findings
WHERE org_id = $1 AND task_id = $2 AND status = 'open' AND severity = 'blocker';
