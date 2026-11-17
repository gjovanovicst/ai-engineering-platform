-- name: UpsertCheckResult :one
INSERT INTO test_results (org_id, repo_id, pull_request_id, run_id, name, status, output, duration_ms, commit_sha, url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (pull_request_id, name)
DO UPDATE SET status = EXCLUDED.status, output = EXCLUDED.output,
              duration_ms = EXCLUDED.duration_ms, commit_sha = EXCLUDED.commit_sha,
              url = EXCLUDED.url, updated_at = now()
RETURNING *;

-- name: ListChecksForPullRequest :many
SELECT * FROM test_results WHERE org_id = $1 AND pull_request_id = $2 ORDER BY name;

-- name: CountFailedChecks :one
SELECT count(*) FROM test_results
WHERE org_id = $1 AND pull_request_id = $2 AND status = 'failed';

-- name: ListChecksForRun :many
SELECT * FROM test_results WHERE org_id = $1 AND run_id = $2 ORDER BY name;

-- name: InsertRunCheck :one
INSERT INTO test_results (org_id, run_id, name, status, output, url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: RequiredChecksForPR :one
SELECT pj.required_checks
FROM pull_requests p
JOIN repos r ON r.id = p.repo_id
JOIN projects pj ON pj.id = r.project_id
WHERE p.id = $1 AND p.org_id = $2;
