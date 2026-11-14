-- +goose Up
ALTER TABLE projects ADD COLUMN policy_allowed_tools text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE projects DROP COLUMN policy_allowed_tools;
