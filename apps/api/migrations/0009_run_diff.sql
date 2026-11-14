-- +goose Up
ALTER TABLE runs ADD COLUMN diff text, ADD COLUMN diff_sha256 text;

-- +goose Down
ALTER TABLE runs DROP COLUMN diff_sha256, DROP COLUMN diff;
