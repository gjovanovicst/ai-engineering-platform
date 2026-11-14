-- +goose Up
ALTER TABLE orgs ADD COLUMN legal_hold boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE orgs DROP COLUMN legal_hold;
