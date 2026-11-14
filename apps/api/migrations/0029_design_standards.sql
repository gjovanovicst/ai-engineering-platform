-- +goose Up
CREATE TABLE design_standards (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name         text NOT NULL,
    version      integer NOT NULL CHECK (version > 0),
    status       text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
    spec         jsonb NOT NULL,
    sha256       bytea NOT NULL,
    published_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name, version)
);
ALTER TABLE design_standards ENABLE ROW LEVEL SECURITY;
CREATE POLICY design_standards_tenant ON design_standards
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION design_standard_immutable() RETURNS trigger AS $$
BEGIN
    IF OLD.status = 'published' AND (
        NEW.status <> 'published' OR NEW.spec <> OLD.spec OR
        NEW.name <> OLD.name OR NEW.version <> OLD.version OR NEW.sha256 <> OLD.sha256
    ) THEN
        RAISE EXCEPTION 'published design standard is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER design_standards_immutable
    BEFORE UPDATE ON design_standards
    FOR EACH ROW EXECUTE FUNCTION design_standard_immutable();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER design_standards_immutable ON design_standards;
DROP FUNCTION design_standard_immutable();
-- +goose StatementEnd
DROP TABLE design_standards;
