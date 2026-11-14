# Worker-driven restore

Status: **implemented and exercised** on 2026-11-11.

`restore-worker` is a small Compose service (docker:27-cli, Docker socket,
`./scripts` mounted read-only) that loops:

```
while true; do sh /scripts/pitr-restore-run.sh; sleep 30; done
```

It claims the oldest pending `restore_requests` row and runs the same runner as
the manual path.

## Verified

```
POST /restores -> pending
worker picks it -> running
restored counts=12/12/330
status=succeeded
```

The runner now calls `pg_switch_wal()` and waits for a new `last_archived_wal`
before fetching; without that, a restore to "now" failed because the current WAL
segment was still unarchived.

## Limits

- one restore at a time; no lease/visibility timeout beyond the `pending` claim
- worker uses the host Docker socket (trusted infrastructure)
- restore is a scratch instance, not a live cutover
- a target after the last archived WAL is switched, but a real WAL gap still
  fails closed
- no UI "cancel" and no progress percent
