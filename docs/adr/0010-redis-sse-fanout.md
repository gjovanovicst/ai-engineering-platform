# ADR-0010: Redis pub/sub for SSE fan-out

**Status:** accepted · 2026-10-24

## Context

docs/02 §11 requires SSE with replay from `run_events` and a Redis `PUBLISH` per
event (architecture §5 step 5). The implementation polled the database every
500 ms, which works on one replica but adds latency and DB load and cannot notify
other API replicas.

## Decision

Use `github.com/redis/go-redis/v9` behind a small `platform/events.Bus`
(Publish/Subscribe). `RunStore.AppendEvent` publishes the inserted row to
`run:{id}`; the SSE handler backfills from the database after `Last-Event-ID` and
then forwards subscriber messages. Without `AIP_REDIS_ADDR` (or when Redis is
unreachable) both sides fall back to the existing 500 ms DB polling, so the
platform still works in a minimal deployment.

## Consequences

- One additional dependency (client only; Redis is already in compose and K8s).
- SSE id remains the `run_events.seq`, so reconnect/replay semantics do not change.
- Publishing is best effort: a failed publish is logged, the DB remains the source
  of truth and the poll fallback covers gaps.
