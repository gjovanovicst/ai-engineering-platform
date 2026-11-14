# Audit integrity design

Status: **option A implemented** · 2026-10-27 · follows docs/31 T10

## Current state

`audit_log` and `run_events` are append-only by application code and by grants
(no UPDATE/DELETE path), but a database administrator can rewrite rows. There is
no external witness.

## Implemented (option A)

Migration 0047 adds `prev_hash`/`hash` to `audit_log`, a BEFORE INSERT trigger that
serialises inserts with an advisory lock and hashes the canonical row, backfills
existing rows, and provides `aip_audit_broken_count()` for verification.

```sql
select aip_audit_broken_count();  -- 0 = chain intact
```

The audit list returns `prev_hash`/`hash`; Settings → Audit has an "Export NDJSON" button, and admins can export NDJSON (`GET /orgs/{org}/audit/export`, up to 10k rows). With `AIP_AUDIT_SIGNING_KEY` set, the response carries `X-AIP-Audit-Signature: sha256=<HMAC-SHA256(body)>`. With `AIP_AUDIT_ED25519_KEY` (base64 seed or private key) set, it carries `ed25519=<signature>` and `GET /api/v1/audit/public-key` returns the public key for verification. Live check: ed25519 verify returned true. HMAC verification: `openssl dgst -sha256 -hmac`. A transaction-insert test appended a row and the chain stayed intact. No tamper
test was run against the live table (updates/deletes are forbidden there).

## Goal

Detect (and preferably prevent) history rewriting without changing the run API.

## Option A — hash chain (recommended first)

Each row gets `prev_hash` and `hash = sha256(prev_hash || canonical(payload))`.
- computed in the same transaction as the insert; the worker/API never updates.
- a verifier job checks the chain and emits `audit.broken` on mismatch.
- cheap, works on Postgres, no new service.
- limitation: an admin can recompute the whole chain; it detects partial edits,
  not a full rewrite.

## Option B — external witness (implemented first slice)

Hourly Merkle root of new audit rows is written to:
- an append-only object store bucket with object lock, and/or
- a second database/account.
Verification compares the local root with the witness.
- detects full rewrites; needs WORM storage and a key/retention policy.

Implemented: `audit_witnesses` + S3 object under `audit-witness/<org>/<ts>.json`;
admins write an anchor (`POST /orgs/{org}/audit/witness`) and verify it
(`GET`, comparing the stored hash at the anchored id). Settings → Audit exposes
both buttons. Live check: write 201, verify `valid=true`. WORM/object-lock and
signed witnesses remain.

## Option C — signed batches

Ed25519-sign the Merkle root with a key not held by the DB admin; store
signatures in the same table plus an external sink. Highest assurance, more key
management.

## Decision

Ship **A now**, design **B** for enterprise; C only if a customer requires it.
Acceptance: a modified/deleted row is detected by the verifier within one run;
the verifier itself cannot repair the chain silently.

## Out of scope

Replacing the audit log, legal hold, retention beyond the existing sweeps.
