-- +goose Up
CREATE TABLE rules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
    level      text NOT NULL CHECK (level IN ('organization','project','module','task')),
    scope_ref  text,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX rules_org_idx ON rules (org_id, project_id, level);
CREATE UNIQUE INDEX rules_name_unique ON rules (org_id, COALESCE(project_id, '00000000-0000-0000-0000-000000000000'::uuid), name);

CREATE TABLE rule_versions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id      uuid NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    version      int NOT NULL,
    body         text NOT NULL,
    sha256       bytea NOT NULL,
    published_at timestamptz,
    published_by uuid REFERENCES users(id),
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rule_id, version)
);

ALTER TABLE rules ENABLE ROW LEVEL SECURITY;
CREATE POLICY rules_tenant ON rules USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

ALTER TABLE rule_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY rule_versions_tenant ON rule_versions USING (
    rule_id IN (SELECT id FROM rules WHERE org_id = app_org_id())
) WITH CHECK (
    rule_id IN (SELECT id FROM rules WHERE org_id = app_org_id())
);

-- +goose Down
DROP TABLE rule_versions, rules;
