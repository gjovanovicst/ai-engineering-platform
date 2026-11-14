-- +goose Up
ALTER TABLE projects ADD COLUMN policy_max_diff_bytes int NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN policy_forbidden_paths text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE projects DROP COLUMN policy_forbidden_paths;
ALTER TABLE projects DROP COLUMN policy_max_diff_bytes;
