-- name: CreateRulePublishApproval :one
INSERT INTO approvals (org_id, type, status, rule_version_id, requested_by, expires_at)
VALUES ($1, 'rule_publish', 'pending', $2, $3, $4)
RETURNING *;

-- name: GetRuleVersion :one
SELECT * FROM rule_versions WHERE id = $1;

-- name: ListPendingRuleApprovals :many
SELECT a.id, a.status, a.requested_at, a.requested_by, a.rule_version_id,
       v.version, v.rule_id, r.name AS rule_name
FROM approvals a
JOIN rule_versions v ON v.id = a.rule_version_id
JOIN rules r ON r.id = v.rule_id
WHERE a.org_id = $1 AND a.type = 'rule_publish' AND a.status = 'pending'
ORDER BY a.requested_at ASC;

-- name: GetRuleApproval :one
SELECT * FROM approvals WHERE id = $1 AND org_id = $2 AND type = 'rule_publish';

-- name: DecideRuleApproval :one
UPDATE approvals
SET status = $3, decided_by = $4, reason = $5, decided_at = now()
WHERE id = $1 AND org_id = $2 AND status = 'pending'
RETURNING *;
