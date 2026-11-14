-- +goose Up
ALTER TABLE orgs ADD COLUMN region text NOT NULL DEFAULT 'eu-central';

-- +goose Down
ALTER TABLE orgs DROP COLUMN region;
