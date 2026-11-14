-- +goose Up
-- Osnova: tenancy, nalozi, sesije. RLS je uključen od prve migracije (ADR-0006).

CREATE TABLE orgs (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
    name                text NOT NULL,
    max_concurrent_runs int NOT NULL DEFAULT 4,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email                citext NOT NULL UNIQUE,
    name                 text NOT NULL,
    password_hash        text,
    avatar_url           text,
    email_verified_at    timestamptz,
    last_login_at        timestamptz,
    must_change_password boolean NOT NULL DEFAULT false,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member','viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE invites (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    email       citext NOT NULL,
    role        text NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member','viewer')),
    token_hash  bytea NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz,
    created_by  uuid NOT NULL REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invites_org_idx ON invites (org_id, created_at DESC);

CREATE TABLE sessions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_hash   bytea NOT NULL UNIQUE,
    platform       text NOT NULL DEFAULT 'web' CHECK (platform IN ('web','mobile','cli')),
    user_agent     text,
    ip             inet,
    expires_at     timestamptz NOT NULL,
    revoked_at     timestamptz,
    revoked_reason text,
    last_used_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE email_verifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE password_resets (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    expires_at   timestamptz NOT NULL,
    used_at      timestamptz,
    requested_ip inet,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_attempts (
    id         bigserial PRIMARY KEY,
    email_hash bytea NOT NULL,
    ip         inet,
    success    boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX login_attempts_email_idx ON login_attempts (email_hash, created_at DESC);

CREATE TABLE user_identities (
    provider   text NOT NULL,
    subject    text NOT NULL,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name         text NOT NULL,
    key_hash     bytea NOT NULL UNIQUE,
    scopes       text[] NOT NULL DEFAULT '{}',
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
    id         bigserial PRIMARY KEY,
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor_id   uuid REFERENCES users(id),
    actor_kind text NOT NULL CHECK (actor_kind IN ('user','api_key','system')),
    action     text NOT NULL,
    target     text NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_org_idx ON audit_log (org_id, created_at DESC);

-- updated_at trigger
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER orgs_set_updated_at BEFORE UPDATE ON orgs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Prava za aplikacionu rolu (nema BYPASSRLS).
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO aip_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO aip_app;

-- audit_log je append-only: bez UPDATE/DELETE.
REVOKE UPDATE, DELETE ON audit_log FROM aip_app;

-- RLS (druga linija odbrane; aplikacija uvek nosi org_id)
ALTER TABLE memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE invites ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY memberships_tenant ON memberships
    USING (org_id = current_setting('app.org_id', true)::uuid);
CREATE POLICY invites_tenant ON invites
    USING (org_id = current_setting('app.org_id', true)::uuid);
CREATE POLICY api_keys_tenant ON api_keys
    USING (org_id = current_setting('app.org_id', true)::uuid);
CREATE POLICY audit_log_tenant ON audit_log
    USING (org_id = current_setting('app.org_id', true)::uuid);

-- +goose Down
DROP TABLE IF EXISTS audit_log, api_keys, user_identities, login_attempts,
    password_resets, email_verifications, sessions, invites, memberships, users, orgs CASCADE;
DROP FUNCTION IF EXISTS set_updated_at();
