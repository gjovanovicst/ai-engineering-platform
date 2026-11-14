-- +goose Up
CREATE TABLE github_apps (
    org_id          uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    app_id          bigint NOT NULL,
    slug            text NOT NULL DEFAULT '',
    pem_encrypted   text NOT NULL,
    secret_encrypted text NOT NULL,
    install_url     text NOT NULL DEFAULT '',
    installation_id bigint,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE github_apps ENABLE ROW LEVEL SECURITY;
CREATE POLICY github_apps_tenant ON github_apps
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

CREATE TABLE github_app_states (
    state      text PRIMARY KEY,
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE github_app_states, github_apps;
