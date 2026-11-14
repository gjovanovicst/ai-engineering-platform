# Migrating the platform database to Patroni

Status: **executed 2026-11-05** (see [48-patroni-cutover.md](48-patroni-cutover.md)); this runbook remains for repeatability. The platform currently uses the single
Postgres behind HAProxy/PgBouncer. `AIP_DB_DSN` is now a Compose variable so the
API/worker can be repointed without editing the file.

## Preconditions

- Patroni cluster is healthy (`/primary` 200 on one node, `/replica` on another)
- HAProxy fronting Patroni is reachable from the API/worker network
- a maintenance window is agreed; writes are stopped during the cutover
- a fresh backup exists (`docs/29-backup-restore.md`)

## Procedure

1. Stop writers: `docker compose stop worker` (API can stay read-only; better stop
   API too).
2. Dump the source:
   `pg_dump --format=custom --no-owner --no-privileges "$SOURCE_DSN" > /tmp/aip.dump`
3. Apply roles/grants to the target (the migration DSN user must exist):
   `psql "$TARGET_ADMIN_DSN" -f deploy/postgres/00_roles.sql`
4. Restore:
   `pg_restore --no-owner --no-privileges --dbname "$TARGET_DSN" /tmp/aip.dump`
5. Run migrations against the target with `aipctl migrate up`.
6. Verify counts and a write:
   - `orgs`, `projects`, `tasks`, `runs`, `audit_log` counts match the source
   - insert/delete a scratch row through the app role
7. Point the platform at the target in `.env`:
   `AIP_DB_DSN=postgres://aip_app:...@<patroni-haproxy>:5432/aip?sslmode=disable`
8. `docker compose up -d --force-recreate api worker`; check `/readyz`, login and a
   planning run.
9. Keep the old database untouched for the rollback window.

## Rollback

- restore the previous `AIP_DB_DSN` and recreate API/worker
- no schema change is made by the cutover itself; migrations must be compatible
  with the old binary if you roll back after step 5

## Not covered

- logical replication/zero-downtime cutover
- cross-region transfer
- credential rotation
