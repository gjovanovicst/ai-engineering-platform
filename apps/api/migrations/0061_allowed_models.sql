-- +goose Up
ALTER TABLE projects ADD COLUMN policy_allowed_models text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE projects DROP COLUMN policy_allowed_models;
