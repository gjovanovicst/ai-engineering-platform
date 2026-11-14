-- name: RecordUsage :one
INSERT INTO usage_ledger (org_id, run_id, provider, model, input_tokens, output_tokens, cost_usd)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListUsageForRun :many
SELECT * FROM usage_ledger WHERE org_id = $1 AND run_id = $2 ORDER BY created_at;
