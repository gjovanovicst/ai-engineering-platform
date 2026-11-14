-- name: ListOpenPullRequestsForSync :many
SELECT pr.id, pr.org_id, pr.repo_id, pr.number, r.provider, r.name AS repo_name
FROM pull_requests pr
JOIN repos r ON r.id = pr.repo_id
WHERE pr.state = 'open'
ORDER BY pr.created_at DESC
LIMIT 50;
