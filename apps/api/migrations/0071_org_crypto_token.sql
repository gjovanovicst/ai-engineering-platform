-- +goose Up
ALTER TABLE org_crypto ADD COLUMN kms_token_enc bytea NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE org_crypto DROP COLUMN kms_token_enc;
