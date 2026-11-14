-- name: CreateInvite :one
INSERT INTO invites (org_id, email, role, token_hash, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListInvites :many
SELECT * FROM invites WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: InviteOrg :one
SELECT app_invite_org($1);

-- name: GetInviteByToken :one
SELECT * FROM invites WHERE org_id = $1 AND token_hash = $2;

-- name: AcceptInvite :one
UPDATE invites SET accepted_at = now()
WHERE id = $1 AND org_id = $2 AND accepted_at IS NULL
RETURNING *;

-- name: UserEmail :one
SELECT email FROM users WHERE id = $1;
