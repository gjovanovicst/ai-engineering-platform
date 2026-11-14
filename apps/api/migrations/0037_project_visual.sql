-- +goose Up
ALTER TABLE projects ADD COLUMN visual_command text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN visual_command;
