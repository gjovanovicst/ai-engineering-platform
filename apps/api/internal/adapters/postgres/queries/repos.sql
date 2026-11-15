-- name: CreateRepo :one
INSERT INTO repos (org_id, project_id, provider, name, external_id, default_branch, webhook_secret)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRepos :many
SELECT * FROM repos WHERE org_id = $1 AND project_id = $2 ORDER BY created_at DESC;

-- name: GetRepo :one
SELECT * FROM repos WHERE id = $1 AND org_id = $2;

-- name: GetRepoForProject :one
SELECT * FROM repos WHERE id = $1 AND org_id = $2 AND project_id = $3;

-- name: RepoOrg :one
SELECT app_repo_org($1);

-- name: CreatePullRequest :one
INSERT INTO pull_requests (org_id, repo_id, task_id, provider, number, url, state)
VALUES ($1, $2, $3, $4, $5, $6, 'open')
RETURNING *;

-- name: PullRequestForMerge :one
SELECT * FROM pull_requests WHERE org_id = $1 AND repo_id = $2 AND number = $3;

-- name: SetPullRequestState :exec
UPDATE pull_requests SET state = $3,
    merged_at = CASE WHEN $3 = 'merged' THEN COALESCE(merged_at, now()) ELSE merged_at END
WHERE id = $1 AND org_id = $2;

-- name: GetRepoByProject :one
SELECT * FROM repos WHERE org_id = $1 AND project_id = $2 ORDER BY created_at ASC LIMIT 1;

-- name: ApprovedPRApproval :one
SELECT id FROM approvals
WHERE org_id = $1 AND pull_request_id = $2 AND type = 'pr' AND status = 'approved'
LIMIT 1;

-- name: ListPullRequestsForProject :many
SELECT pr.* FROM pull_requests pr
JOIN repos r ON r.id = pr.repo_id
WHERE pr.org_id = $1 AND r.project_id = $2
ORDER BY pr.created_at DESC LIMIT $3;

-- name: GetPullRequest :one
SELECT pr.*, t.ref AS task_ref, t.title AS task_title, r.name AS repo_name
FROM pull_requests pr
JOIN tasks t ON t.id = pr.task_id
JOIN repos r ON r.id = pr.repo_id
WHERE pr.id = $1 AND pr.org_id = $2;

-- name: GetRepoByName :one
SELECT * FROM repos WHERE org_id = $1 AND provider = $2 AND name = $3;
