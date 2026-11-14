-- +goose Up
ALTER TABLE projects ADD COLUMN policy_change_window text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN policy_change_window;
