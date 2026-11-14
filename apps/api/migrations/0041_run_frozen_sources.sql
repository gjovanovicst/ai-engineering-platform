-- +goose Up
ALTER TABLE runs ADD COLUMN frozen_context_sources text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE runs DROP COLUMN frozen_context_sources;
