#!/bin/sh
# Restore the newest dump into a scratch database and verify it is readable.
# Used by the monthly restore drill; never points at production.
set -eu

DSN=${AIP_DB_DSN:?AIP_DB_DSN is required}
DUMP=${1:?usage: restore-check.sh <dump file>}
SCRATCH=${SCRATCH_DB:-aip_restore_check}

echo "[restore] dropping and recreating $SCRATCH"
psql "$DSN" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS $SCRATCH"
psql "$DSN" -v ON_ERROR_STOP=1 -c "CREATE DATABASE $SCRATCH"

BASE=$(echo "$DSN" | sed "s#/aip\([?].*\)\{0,1\}\$#/$SCRATCH\1#")
pg_restore --no-owner --no-privileges --dbname="$BASE" "$DUMP"

echo "[restore] sanity queries"
psql "$BASE" -v ON_ERROR_STOP=1 -tAc "select 'orgs=' || count(*) from orgs"
psql "$BASE" -v ON_ERROR_STOP=1 -tAc "select 'runs=' || count(*) from runs"
psql "$BASE" -v ON_ERROR_STOP=1 -tAc "select 'migrations=' || count(*) from goose_db_version"

echo "[restore] OK ($SCRATCH)"
