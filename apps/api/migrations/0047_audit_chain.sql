-- +goose Up
ALTER TABLE audit_log ADD COLUMN prev_hash text NOT NULL DEFAULT '';
ALTER TABLE audit_log ADD COLUMN hash text NOT NULL DEFAULT '';

-- +goose StatementBegin
CREATE FUNCTION aip_audit_canonical(prev text, row_id bigint, org uuid, actor uuid, actor_kind text, action text, target text, metadata jsonb, created timestamptz)
RETURNS text LANGUAGE sql IMMUTABLE AS $$
    SELECT concat_ws('|', prev, row_id::text, org::text, COALESCE(actor::text, ''), actor_kind, action, target,
        metadata::text, to_char(created AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'))
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION aip_audit_chain() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE prev text;
BEGIN
    PERFORM pg_advisory_xact_lock(947211);
    SELECT hash INTO prev FROM audit_log ORDER BY id DESC LIMIT 1;
    NEW.prev_hash := COALESCE(prev, '');
    NEW.hash := encode(sha256(convert_to(aip_audit_canonical(NEW.prev_hash, NEW.id, NEW.org_id, NEW.actor_id,
        NEW.actor_kind, NEW.action, NEW.target, NEW.metadata, NEW.created_at), 'UTF8')), 'hex');
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_log_chain BEFORE INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION aip_audit_chain();

-- Backfill the existing rows in id order.
-- +goose StatementBegin
DO $$
DECLARE r record; prev text := ''; h text;
BEGIN
    FOR r IN SELECT * FROM audit_log ORDER BY id LOOP
        h := encode(sha256(convert_to(aip_audit_canonical(prev, r.id, r.org_id, r.actor_id, r.actor_kind,
            r.action, r.target, r.metadata, r.created_at), 'UTF8')), 'hex');
        UPDATE audit_log SET prev_hash = prev, hash = h WHERE id = r.id;
        prev := h;
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION aip_audit_broken_count() RETURNS bigint LANGUAGE sql SECURITY DEFINER SET search_path = public AS $$
    WITH chained AS (
        SELECT *, lag(hash) OVER (ORDER BY id) AS expected_prev FROM audit_log
    )
    SELECT count(*)::bigint FROM chained
    WHERE prev_hash <> COALESCE(expected_prev, '')
       OR hash <> encode(sha256(convert_to(aip_audit_canonical(prev_hash, id, org_id, actor_id, actor_kind,
            action, target, metadata, created_at), 'UTF8')), 'hex')
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION aip_audit_broken_count();
DROP TRIGGER audit_log_chain ON audit_log;
DROP FUNCTION aip_audit_chain();
DROP FUNCTION aip_audit_canonical(text, bigint, uuid, uuid, text, text, text, jsonb, timestamptz);
ALTER TABLE audit_log DROP COLUMN hash;
ALTER TABLE audit_log DROP COLUMN prev_hash;
