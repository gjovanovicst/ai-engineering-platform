-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, email_verified_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UserByID :one
SELECT * FROM users WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = $2 WHERE id = $1 AND email_verified_at IS NULL;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: CreateOrg :one
INSERT INTO orgs (slug, name) VALUES ($1, $2) RETURNING *;

-- name: AddMembership :exec
INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, $3);

-- name: PrimaryMembership :one
SELECT org_id, role
FROM memberships
WHERE user_id = $1
ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END
LIMIT 1;

-- name: CreateVerification :exec
INSERT INTO email_verifications (user_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: ConsumeVerification :one
UPDATE email_verifications
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING user_id;

-- name: CreatePasswordReset :exec
INSERT INTO password_resets (user_id, token_hash, expires_at, requested_ip) VALUES ($1, $2, $3, $4);

-- name: ConsumePasswordReset :one
UPDATE password_resets
SET used_at = $2
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING user_id;

-- name: InsertAudit :exec
INSERT INTO audit_log (org_id, actor_id, actor_kind, action, target, metadata)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: OrgSlugExists :one
SELECT EXISTS (SELECT 1 FROM orgs WHERE slug = $1);

-- name: UserIDByIdentity :one
SELECT user_id FROM user_identities WHERE provider = $1 AND subject = $2;

-- name: CreateIdentity :exec
INSERT INTO user_identities (provider, subject, user_id) VALUES ($1, $2, $3)
ON CONFLICT (provider, subject) DO NOTHING;

-- name: SetMembershipRole :exec
UPDATE memberships SET role = $3
WHERE user_id = $1 AND org_id = $2 AND role <> 'owner';
