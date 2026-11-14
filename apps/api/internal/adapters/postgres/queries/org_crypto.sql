-- name: GetOrgCrypto :one
SELECT kms_url, key_id, kms_token_enc, status, updated_at FROM org_crypto WHERE org_id = $1;

-- name: UpsertOrgCrypto :one
INSERT INTO org_crypto (org_id, kms_url, key_id, kms_token_enc, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (org_id) DO UPDATE
SET kms_url = EXCLUDED.kms_url, key_id = EXCLUDED.key_id, kms_token_enc = EXCLUDED.kms_token_enc, status = 'active', updated_at = now()
RETURNING kms_url, key_id, kms_token_enc, status, updated_at;

-- name: DisableOrgCrypto :exec
UPDATE org_crypto SET status = 'disabled', updated_at = now() WHERE org_id = $1;
