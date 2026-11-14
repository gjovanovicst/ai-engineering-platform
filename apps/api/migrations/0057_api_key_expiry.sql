-- +goose Up
ALTER TABLE api_keys ADD COLUMN expires_at timestamptz;

-- +goose Down
ALTER TABLE api_keys DROP COLUMN expires_at;
