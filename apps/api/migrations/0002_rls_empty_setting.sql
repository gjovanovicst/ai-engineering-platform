-- +goose Up
-- current_setting('app.org_id', true) returns an empty string (not NULL) once the
-- GUC has been set at least once in a session, and ''::uuid raises an error.
-- Without this helper, a query outside WithOrg fails instead of returning zero rows.
-- +goose StatementBegin
CREATE FUNCTION app_org_id() RETURNS uuid
LANGUAGE sql STABLE AS $$
    SELECT nullif(current_setting('app.org_id', true), '')::uuid
$$;
-- +goose StatementEnd

DROP POLICY memberships_tenant ON memberships;
CREATE POLICY memberships_tenant ON memberships
    USING (org_id = app_org_id());

DROP POLICY invites_tenant ON invites;
CREATE POLICY invites_tenant ON invites
    USING (org_id = app_org_id());

DROP POLICY api_keys_tenant ON api_keys;
CREATE POLICY api_keys_tenant ON api_keys
    USING (org_id = app_org_id());

DROP POLICY audit_log_tenant ON audit_log;
CREATE POLICY audit_log_tenant ON audit_log
    USING (org_id = app_org_id());

-- +goose Down
DROP POLICY memberships_tenant ON memberships;
CREATE POLICY memberships_tenant ON memberships
    USING (org_id = current_setting('app.org_id', true)::uuid);

DROP POLICY invites_tenant ON invites;
CREATE POLICY invites_tenant ON invites
    USING (org_id = current_setting('app.org_id', true)::uuid);

DROP POLICY api_keys_tenant ON api_keys;
CREATE POLICY api_keys_tenant ON api_keys
    USING (org_id = current_setting('app.org_id', true)::uuid);

DROP POLICY audit_log_tenant ON audit_log;
CREATE POLICY audit_log_tenant ON audit_log
    USING (org_id = current_setting('app.org_id', true)::uuid);

DROP FUNCTION app_org_id();
