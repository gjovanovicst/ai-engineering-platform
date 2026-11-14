-- name: CreateRun :one
INSERT INTO runs (org_id, project_id, task_id, agent_type, status, model, budget, frozen_context_hash, frozen_context_sources)
VALUES ($1, $2, $3, $4, 'queued', $5, $6, $7, $8)
RETURNING *;

-- name: GetRun :one
SELECT * FROM runs WHERE id = $1 AND org_id = $2;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = $3, current_phase = $4, failure_code = $5, failure_detail = $6,
    cancel_requested_at = $7, started_at = $8, finished_at = $9
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: LockRun :one
SELECT * FROM runs WHERE id = $1 AND org_id = $2 FOR UPDATE;

-- name: InsertRunEvent :one
INSERT INTO run_events (run_id, seq, type, actor, payload)
SELECT $1, COALESCE(MAX(seq), 0) + 1, $2, $3, $4
FROM run_events WHERE run_id = $1
RETURNING *;

-- name: ListRunEvents :many
SELECT * FROM run_events WHERE run_id = $1 ORDER BY seq;

-- name: ListRuns :many
SELECT * FROM runs WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListRunEventsAfter :many
SELECT * FROM run_events WHERE run_id = $1 AND seq > $2 ORDER BY seq;

-- name: SetRunPlan :exec
UPDATE runs SET plan = $3, plan_sha256 = $4 WHERE id = $1 AND org_id = $2;

-- name: CreateApproval :one
INSERT INTO approvals (org_id, run_id, pull_request_id, type, status, expires_at)
VALUES ($1, $2, $3, $4, 'pending', $5)
RETURNING *;

-- name: GetApproval :one
SELECT a.*, pr.number AS pr_number, pr.url AS pr_url
FROM approvals a
LEFT JOIN pull_requests pr ON pr.id = a.pull_request_id
WHERE a.id = $1 AND a.org_id = $2;

-- name: PendingApprovalForRun :one
SELECT * FROM approvals WHERE run_id = $1 AND org_id = $2 AND status = 'pending' LIMIT 1;

-- name: DecideApproval :one
UPDATE approvals
SET status = $3, decided_at = $4, decided_by = $5, reason = $6
WHERE id = $1 AND org_id = $2 AND status = 'pending'
RETURNING *;

-- name: ListPendingApprovals :many
SELECT a.*, pr.number AS pr_number, pr.url AS pr_url
FROM approvals a
LEFT JOIN pull_requests pr ON pr.id = a.pull_request_id
WHERE a.org_id = $1 AND a.status = 'pending'
ORDER BY a.requested_at DESC
LIMIT $2;

-- name: SetRunDiff :exec
UPDATE runs SET diff = $3, diff_sha256 = $4 WHERE id = $1 AND org_id = $2;

-- name: SetRunReview :exec
UPDATE runs SET review = $3 WHERE id = $1 AND org_id = $2;

-- name: LatestRunForTask :one
SELECT * FROM runs
WHERE org_id = $1 AND task_id = $2 AND agent_type = $3 AND status = 'succeeded'
ORDER BY created_at DESC LIMIT 1;

-- name: ListRunsForTask :many
SELECT * FROM runs WHERE org_id = $1 AND task_id = $2 ORDER BY created_at DESC LIMIT $3;

-- name: ExpireApprovals :many
UPDATE approvals
SET status = 'expired', decided_at = now()
WHERE org_id = $1 AND status = 'pending' AND expires_at < now()
RETURNING id, run_id, type;

-- name: CountActiveRuns :one
SELECT count(*) FROM runs WHERE org_id = $1 AND status IN ('queued','running');

-- name: LatestReviewForTask :one
SELECT review FROM runs
WHERE org_id = $1 AND task_id = $2 AND agent_type = 'review' AND review IS NOT NULL
ORDER BY created_at DESC LIMIT 1;

-- name: SetRunAcceptanceProof :exec
UPDATE runs SET acceptance_proof = $3 WHERE id = $1 AND org_id = $2;

-- name: SetRunContextHash :exec
UPDATE runs SET context_hash = $3 WHERE id = $1 AND org_id = $2;

-- name: LatestRunForProject :one
SELECT * FROM runs
WHERE org_id = $1 AND project_id = $2 AND agent_type = 'development'
ORDER BY created_at DESC LIMIT 1;

-- name: SetRunTraceID :exec
UPDATE runs SET trace_id = $3 WHERE id = $1 AND org_id = $2;
