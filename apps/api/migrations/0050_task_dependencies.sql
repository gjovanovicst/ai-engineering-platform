-- +goose Up
ALTER TABLE tasks ADD COLUMN depends_on uuid[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE tasks DROP COLUMN depends_on;
