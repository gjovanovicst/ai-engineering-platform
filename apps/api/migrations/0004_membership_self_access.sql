-- +goose Up
-- A user must be able to list their own memberships before any org context
-- exists (login). The policy is additive: OR-ed with memberships_tenant.
-- +goose StatementBegin
CREATE FUNCTION app_user_id() RETURNS uuid
LANGUAGE sql STABLE AS $$
    SELECT nullif(current_setting('app.user_id', true), '')::uuid
$$;
-- +goose StatementEnd

CREATE POLICY memberships_self ON memberships
    USING (user_id = app_user_id());

-- +goose Down
DROP POLICY memberships_self ON memberships;
DROP FUNCTION app_user_id();
