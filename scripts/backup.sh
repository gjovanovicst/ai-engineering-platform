#!/bin/sh
# Logical backup with retention. POSIX sh so it runs in the postgres alpine
# image as well as on a host. Requires pg_dump and sha256sum.
set -eu

DSN=${AIP_DB_DSN:?AIP_DB_DSN is required}
OUT_DIR=${BACKUP_DIR:-/backups}
RETENTION_DAYS=${RETENTION_DAYS:-14}
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
FILE="$OUT_DIR/aip-$STAMP.dump"

mkdir -p "$OUT_DIR"
echo "[backup] dumping to $FILE"
pg_dump --format=custom --no-owner --no-privileges --file="$FILE" "$DSN"
sha256sum "$FILE" > "$FILE.sha256"

# Keep the retention window; never delete the newest file.
find "$OUT_DIR" -name 'aip-*.dump' -type f -mtime +"$RETENTION_DAYS" -delete
find "$OUT_DIR" -name 'aip-*.dump.sha256' -type f -mtime +"$RETENTION_DAYS" -delete

echo "[backup] done: $(du -h "$FILE" | cut -f1)"
