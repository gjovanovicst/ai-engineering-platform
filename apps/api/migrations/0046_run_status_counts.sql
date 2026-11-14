-- +goose Up
CREATE OR REPLACE FUNCTION aip_run_status_counts()
RETURNS TABLE(status text, count bigint)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT runs.status, count(*)::bigint FROM runs GROUP BY runs.status
$$;
GRANT EXECUTE ON FUNCTION aip_run_status_counts() TO aip_app;

-- +goose Down
DROP FUNCTION aip_run_status_counts();
