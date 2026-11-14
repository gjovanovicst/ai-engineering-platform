-- +goose Up
ALTER TABLE org_sso ADD COLUMN sso_url text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE org_sso DROP COLUMN sso_url;
