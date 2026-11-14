-- +goose Up
ALTER TABLE blueprints ADD COLUMN status text NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','published'));
ALTER TABLE blueprints ADD COLUMN published_at timestamptz;

-- projects.blueprint_id already exists (0005); add the FK now that blueprints exists.
ALTER TABLE projects ADD CONSTRAINT projects_blueprint_fk
    FOREIGN KEY (blueprint_id) REFERENCES blueprints(id) ON DELETE SET NULL;
CREATE INDEX projects_blueprint_idx ON projects (blueprint_id) WHERE blueprint_id IS NOT NULL;

-- A published blueprint version is immutable: no UPDATE of spec once published.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION blueprint_spec_immutable() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.spec IS DISTINCT FROM OLD.spec THEN
        RAISE EXCEPTION 'published blueprint spec is immutable';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER blueprints_spec_immutable
    BEFORE UPDATE ON blueprints
    FOR EACH ROW EXECUTE FUNCTION blueprint_spec_immutable();

-- +goose Down
DROP TRIGGER blueprints_spec_immutable ON blueprints;
DROP FUNCTION blueprint_spec_immutable();
DROP INDEX projects_blueprint_idx;
ALTER TABLE projects DROP CONSTRAINT projects_blueprint_fk;
ALTER TABLE blueprints DROP COLUMN published_at, DROP COLUMN status;
