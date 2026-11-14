-- +goose Up
CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       text NOT NULL,
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    link       text NOT NULL DEFAULT '',
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY notifications_tenant ON notifications USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE notifications;
