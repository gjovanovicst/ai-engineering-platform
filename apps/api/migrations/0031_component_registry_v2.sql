-- +goose Up
ALTER TABLE component_entries
    ADD COLUMN category text NOT NULL DEFAULT 'other',
    ADD COLUMN framework text NOT NULL DEFAULT '',
    ADD COLUMN package text NOT NULL DEFAULT '',
    ADD COLUMN import_path text NOT NULL DEFAULT '',
    ADD COLUMN version text NOT NULL DEFAULT '',
    ADD COLUMN docs text NOT NULL DEFAULT '',
    ADD COLUMN example text NOT NULL DEFAULT '';

ALTER TABLE component_entries DROP CONSTRAINT component_entries_status_check;
ALTER TABLE component_entries
    ADD CONSTRAINT component_entries_status_check
    CHECK (status IN ('draft','published','approved','deprecated','forbidden'));

ALTER TABLE component_entries
    ADD CONSTRAINT component_entries_category_check
    CHECK (category IN ('layout','navigation','data','form','feedback','overlay','typography','other'));

-- +goose Down
ALTER TABLE component_entries DROP CONSTRAINT component_entries_category_check;
ALTER TABLE component_entries DROP CONSTRAINT component_entries_status_check;
ALTER TABLE component_entries ADD CONSTRAINT component_entries_status_check
    CHECK (status IN ('draft','published','deprecated'));
ALTER TABLE component_entries
    DROP COLUMN category, DROP COLUMN framework, DROP COLUMN package,
    DROP COLUMN import_path, DROP COLUMN version, DROP COLUMN docs, DROP COLUMN example;
