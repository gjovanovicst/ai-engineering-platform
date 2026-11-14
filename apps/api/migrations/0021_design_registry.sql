-- +goose Up
CREATE TABLE design_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name       text NOT NULL,
    value      text NOT NULL,
    category   text NOT NULL CHECK (category IN ('color','typography','spacing','radius','layout','other')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE TABLE component_entries (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name       text NOT NULL,
    purpose    text NOT NULL DEFAULT '',
    rules      text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','deprecated')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

ALTER TABLE design_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY design_tokens_tenant ON design_tokens USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());
ALTER TABLE component_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY component_entries_tenant ON component_entries USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE component_entries, design_tokens;
