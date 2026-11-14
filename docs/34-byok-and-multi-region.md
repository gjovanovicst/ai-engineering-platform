# BYOK and multi-region — design

Status: **first BYOK slice implemented** (org_crypto, v4 per-org KMS, resolver, admin API). Multi-region remains design only.
Current behavior: provider secrets use envelope encryption with a deployment-wide
master key; `AIP_KMS_URL` may wrap DEKs; `orgs.region` is a stored label with no
routing. See `docs/32-audit-integrity.md` and `apps/api/internal/app/providers`.

## 1. Why

Enterprise buyers ask for two things that cannot be faked:

1. **BYOK** — a customer-controlled KMS key wraps their data keys, so revoking the
   key makes the data unreadable.
2. **Residency** — data is stored and processed in a declared region, not just
   labelled.

## 1a. Implemented slice (2026-11-04)

- migration 0070: `org_crypto` (kms_url, key_id, active/disabled, RLS)
- admin `GET/PUT/DELETE /orgs/{org}/crypto`; `audit` on enable/disable
- provider cipher resolver: active org config → a `Cipher` bound to that KMS/key
  (version 4 envelope); no config → deployment cipher
- v4 layout: `0x04 | u16 keyIdLen | keyId | u16 wrappedLen | wrapped | payload`
- live drill with kms-dev: enabled BYOK, stored a provider key (first byte `0x04`,
  107 bytes), deleted the provider, disabled BYOK; `org_crypto` row removed
- Settings → Encryption exposes enable/disable, status and an optional KMS token;
  the token is encrypted with the deployment key (`kms_token_enc`) and decrypted
  only in the resolver, never returned by the API
- live: token ciphertext 99 bytes, no plaintext; provider stored as v4 (`0x04`,
  108 bytes)
- revoke drill verified: after disable, provider key creation returns 400
  `provider key storage is unavailable: BYOK is disabled for this organisation`
- not implemented: per-org mTLS/HSM-backed token wrapping, fallback for v2 rows
  when BYOK is disabled (disabled currently fails all provider key operations)

## 2. BYOK design

### 2.1 Key hierarchy

```
customer KMS key (per org, per region)
  └─ wraps per-secret DEK (provider API keys)
       └─ AES-256-GCM encrypts the secret
```

The deployment master key remains only for legacy rows and for orgs without
BYOK. A BYOK org must not fall back silently.

### 2.2 Schema (not migrated yet)

```sql
CREATE TABLE org_crypto (
  org_id       uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
  provider     text NOT NULL,          -- 'aws-kms' | 'gcp-kms' | 'http-kms'
  key_id       text NOT NULL,          -- ARN / resource name
  region       text NOT NULL,
  status       text NOT NULL,          -- 'active' | 'disabled' | 'revoked'
  created_at   timestamptz NOT NULL DEFAULT now()
);
```

`providers.api_key_enc` gains a key version prefix (v4) that records
`org_crypto.key_id`; v2/v3 rows remain readable by the deployment key.

### 2.3 Write/read path

- **Write**: resolve `org_crypto`; if active, call the customer KMS `Encrypt`
  with the org key and store v4. If the KMS fails, refuse the write — never store
  plaintext or fall back.
- **Read**: v4 → customer KMS `Decrypt`; v2/v3 → deployment key/KMS.
- **Revoke**: setting `status=revoked` must make all v4 reads fail and surface a
  clear error; no automatic plaintext export.
- **Rotate**: re-wrap DEKs under a new key id in a background job, with a
  per-row audit event; never rewrite history of `audit_log`.

### 2.4 Failure modes

| Failure | Required behavior |
|---|---|
| Customer KMS unreachable | writes fail closed; reads fail with a typed error |
| Key revoked/disabled | v4 reads fail; UI shows the org as crypto-locked |
| Wrong key id | decrypt fails; no fallback |
| Cross-region key | reject at configuration time |

## 3. Multi-region design

- `orgs.region` stays the declaration; it must not be presented as enforcement.
- A data plane per region (Postgres + object store + worker pool) with an
  org→region routing table; cross-region requests are rejected, not proxied.
- Audit/witness objects are written in the org region; a global control plane
  stores only IDs and routing metadata.
- Migration between regions is an explicit export/import with a maintenance
  window and a new witness; no silent copy.

## 4. Rollout

1. `org_crypto` + v4 format behind a feature flag; shadow-read check.
2. Per-org opt-in; refuse writes if BYOK is enabled but unavailable.
3. Key rotation drill and a revoke drill (reads must fail).
4. Multi-region only after a second data plane exists; do not ship a routing
   table without it.

## 5. Test plan (before claiming done)

- v4 round-trip with a fake KMS; v2/v3 legacy decrypt.
- KMS outage → write fails, no plaintext row.
- revoke → read fails with a typed error; audit event recorded.
- rotation job re-wraps without changing plaintext.
- cross-region rejection and a documented residency test.
