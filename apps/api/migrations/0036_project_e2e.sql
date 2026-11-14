-- +goose Up
ALTER TABLE projects ADD COLUMN e2e_command text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN e2e_command;
