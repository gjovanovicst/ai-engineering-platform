-- +goose Up
CREATE TABLE restore_requests (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    target_time  timestamptz NOT NULL,
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','succeeded','failed')),
    requested_by uuid REFERENCES users(id) ON DELETE SET NULL,
    detail       text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE restore_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY restore_requests_tenant ON restore_requests
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());
CREATE INDEX restore_requests_org_created ON restore_requests (org_id, created_at DESC);

-- +goose Down
DROP TABLE restore_requests;
