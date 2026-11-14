-- +goose Up
ALTER TABLE projects ADD COLUMN plugin_checks text NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE projects DROP COLUMN plugin_checks;
