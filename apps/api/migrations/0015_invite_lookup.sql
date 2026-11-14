-- +goose Up
-- Invite acceptance needs the org before a session exists in that org.
CREATE FUNCTION app_invite_org(hash bytea) RETURNS uuid
    LANGUAGE sql STABLE SECURITY DEFINER
    SET search_path = public
AS $$ SELECT org_id FROM invites WHERE token_hash = hash $$;

GRANT EXECUTE ON FUNCTION app_invite_org(bytea) TO aip_app;

-- +goose Down
DROP FUNCTION app_invite_org(bytea);
