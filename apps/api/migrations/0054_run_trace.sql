-- +goose Up
ALTER TABLE runs ADD COLUMN trace_id text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE runs DROP COLUMN trace_id;
