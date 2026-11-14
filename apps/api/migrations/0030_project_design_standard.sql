-- +goose Up
ALTER TABLE projects
    ADD COLUMN design_standard_version integer;
ALTER TABLE projects
    ADD CONSTRAINT projects_design_system_fk
    FOREIGN KEY (design_system_id) REFERENCES design_standards(id) ON DELETE SET NULL;
CREATE INDEX projects_design_system_idx ON projects (design_system_id);

-- +goose Down
DROP INDEX projects_design_system_idx;
ALTER TABLE projects DROP CONSTRAINT projects_design_system_fk;
ALTER TABLE projects DROP COLUMN design_standard_version;
