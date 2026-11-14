-- +goose Up
CREATE TABLE task_attachments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    task_id      uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    filename     text NOT NULL,
    content_type text NOT NULL,
    size_bytes   int NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
    sha256       text NOT NULL,
    data         bytea NOT NULL,
    uploaded_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX task_attachments_task_idx ON task_attachments(org_id, task_id, created_at);
ALTER TABLE task_attachments ENABLE ROW LEVEL SECURITY;
CREATE POLICY task_attachments_tenant ON task_attachments
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE task_attachments;
