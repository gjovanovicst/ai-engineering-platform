-- name: UpsertModelPrice :one
INSERT INTO model_prices (org_id, provider, model, input_per_1k, output_per_1k)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (org_id, provider, model)
DO UPDATE SET input_per_1k = EXCLUDED.input_per_1k, output_per_1k = EXCLUDED.output_per_1k, updated_at = now()
RETURNING *;

-- name: ListModelPrices :many
SELECT * FROM model_prices WHERE org_id = $1 ORDER BY provider, model;

-- name: GetModelPrice :one
SELECT * FROM model_prices WHERE org_id = $1 AND provider = $2 AND model = $3;
