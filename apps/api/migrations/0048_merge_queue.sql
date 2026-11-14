-- +goose Up
CREATE TABLE merge_queue (
    id         bigserial PRIMARY KEY,
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    run_id     uuid REFERENCES runs(id) ON DELETE SET NULL,
    task_id    uuid REFERENCES tasks(id) ON DELETE SET NULL,
    number     int NOT NULL,
    status     text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','merging','merged','failed','canceled')),
    error      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX merge_queue_active_idx ON merge_queue (project_id, status, id);
ALTER TABLE merge_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY merge_queue_tenant ON merge_queue
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE merge_queue;
