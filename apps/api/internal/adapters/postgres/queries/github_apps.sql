-- name: UpsertGithubApp :one
INSERT INTO github_apps (org_id, app_id, slug, pem_encrypted, secret_encrypted, install_url, installation_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (org_id) DO UPDATE SET
    app_id = EXCLUDED.app_id, slug = EXCLUDED.slug, pem_encrypted = EXCLUDED.pem_encrypted,
    secret_encrypted = EXCLUDED.secret_encrypted, install_url = EXCLUDED.install_url,
    installation_id = EXCLUDED.installation_id, updated_at = now()
RETURNING *;

-- name: GetGithubApp :one
SELECT * FROM github_apps WHERE org_id = $1;

-- name: SetGithubInstallation :one
UPDATE github_apps SET installation_id = $2, updated_at = now() WHERE org_id = $1 RETURNING *;

-- name: CreateGithubAppState :exec
INSERT INTO github_app_states (state, org_id) VALUES ($1, $2);

-- name: GetGithubAppState :one
SELECT state, org_id FROM github_app_states WHERE state = $1;

-- name: DeleteGithubAppState :exec
DELETE FROM github_app_states WHERE state = $1;
