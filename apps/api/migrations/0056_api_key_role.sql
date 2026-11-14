-- +goose Up
ALTER TABLE api_keys ADD COLUMN role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member','viewer'));
ALTER TABLE api_keys ADD COLUMN revoked_at timestamptz;

-- +goose Down
ALTER TABLE api_keys DROP COLUMN revoked_at;
ALTER TABLE api_keys DROP COLUMN role;
