-- name: CreateScimToken :one
INSERT INTO scim_tokens (org_id, name, token_hash) VALUES ($1, $2, $3)
RETURNING id, name, created_at;

-- name: ListScimTokens :many
SELECT id, name, created_at, revoked_at FROM scim_tokens
WHERE org_id = $1 ORDER BY created_at DESC;

-- name: RevokeScimToken :execrows
UPDATE scim_tokens SET revoked_at = now()
WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;

-- name: ScimTokenByHash :one
SELECT org_id FROM scim_tokens
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: ListScimUsers :many
SELECT u.id, u.email, u.name, u.created_at
FROM users u
JOIN memberships m ON m.user_id = u.id
WHERE m.org_id = $1
ORDER BY u.email;

-- name: SCIMOwner :one
SELECT user_id FROM memberships WHERE org_id = $1 AND role = 'owner' ORDER BY created_at LIMIT 1;

-- name: SCIMMembershipRole :one
SELECT role FROM memberships WHERE org_id = $1 AND user_id = $2;

-- name: SCIMRemoveMembership :exec
DELETE FROM memberships WHERE org_id = $1 AND user_id = $2;

-- name: SCIMUpdateMembershipRole :execrows
UPDATE memberships SET role = $3 WHERE org_id = $1 AND user_id = $2;

-- name: SCIMUpdateUserName :execrows
UPDATE users SET name = $3
WHERE id = $2 AND EXISTS (SELECT 1 FROM memberships m WHERE m.org_id = $1 AND m.user_id = users.id);

-- name: SCIMUpdateUserEmail :execrows
UPDATE users SET email = $3
WHERE id = $2 AND EXISTS (SELECT 1 FROM memberships m WHERE m.org_id = $1 AND m.user_id = users.id);
