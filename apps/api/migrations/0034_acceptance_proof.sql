-- +goose Up
ALTER TABLE runs ADD COLUMN acceptance_proof jsonb;

-- +goose Down
ALTER TABLE runs DROP COLUMN acceptance_proof;
