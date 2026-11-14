-- +goose Up
CREATE TABLE tasks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id     uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    ref            text NOT NULL,
    title          text NOT NULL,
    description    text NOT NULL DEFAULT '',
    acceptance     jsonb NOT NULL DEFAULT '[]',
    status         text NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','ready','planning','in_progress','review','done','blocked','canceled')),
    priority       text NOT NULL DEFAULT 'med' CHECK (priority IN ('low','med','high')),
    tags           text[] NOT NULL DEFAULT '{}',
    assignee_id    uuid REFERENCES users(id),
    parent_id      uuid REFERENCES tasks(id),
    blocked_reason text,
    created_by     uuid REFERENCES users(id),
    ready_at       timestamptz,
    done_at        timestamptz,
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, ref)
);
CREATE INDEX tasks_project_status_idx ON tasks (project_id, status, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER tasks_set_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tasks_tenant ON tasks
    USING (org_id = app_org_id())
    WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE tasks;
