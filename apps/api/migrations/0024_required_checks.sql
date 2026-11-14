-- +goose Up
ALTER TABLE projects ADD COLUMN required_checks text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE projects DROP COLUMN required_checks;
