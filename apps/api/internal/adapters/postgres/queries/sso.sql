-- name: GetOrgSSO :one
SELECT entity_id, metadata_url, certificate_pem, sso_url, enabled, updated_at FROM org_sso WHERE org_id = $1;

-- name: UpsertOrgSSO :one
INSERT INTO org_sso (org_id, entity_id, metadata_url, certificate_pem, sso_url, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (org_id) DO UPDATE
SET entity_id = EXCLUDED.entity_id, metadata_url = EXCLUDED.metadata_url,
    certificate_pem = EXCLUDED.certificate_pem, sso_url = EXCLUDED.sso_url,
    enabled = EXCLUDED.enabled, updated_at = now()
RETURNING entity_id, metadata_url, certificate_pem, sso_url, enabled, updated_at;
