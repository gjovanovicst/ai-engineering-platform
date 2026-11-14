-- +goose Up
ALTER TABLE runs ADD COLUMN context_hash text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN frozen_context_hash text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE runs DROP COLUMN frozen_context_hash;
ALTER TABLE runs DROP COLUMN context_hash;
