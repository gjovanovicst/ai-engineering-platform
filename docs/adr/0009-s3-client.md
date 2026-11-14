# ADR-0009: MinIO Go client for the artifact store

**Status:** accepted · 2026-10-23

## Context

`run_events` older than 90 days and run artifacts (diff, review, test output)
must leave the hot database (docs/03 §6). The composition already ships MinIO
(`minio` service in `docker-compose.yml`, `AIP_S3_*` in `.env.example`), which
speaks the S3 API.

## Decision

Use `github.com/minio/minio-go/v7` behind a small `platform/artifacts.Store`
interface (`Put`). The worker creates the bucket when it is missing. When
`AIP_S3_ENDPOINT` is empty the store is nil and archiving is a logged no-op.

## Consequences

- One additional dependency (and its transitive modules) with an S3-compatible
  API; it works against MinIO, AWS S3 and other S3 implementations.
- Archiving is append-only: events are copied as NDJSON (gzip) and **not deleted**
  from the database, because `run_events` is append-only by rule (AGENTS.md §4).
  A future operator-run purge can remove them after a retention decision.
