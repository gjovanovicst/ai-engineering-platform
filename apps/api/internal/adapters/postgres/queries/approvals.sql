
-- name: UpsertApprovalVote :exec
INSERT INTO approval_votes (org_id, approval_id, user_id, decision, reason)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (approval_id, user_id) DO UPDATE SET decision = EXCLUDED.decision, reason = EXCLUDED.reason;

-- name: CountApprovalVotes :one
SELECT count(*)::bigint FROM approval_votes WHERE approval_id = $1 AND decision = 'approved';
