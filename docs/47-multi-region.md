# Multi-region design

Status: **first routing guard implemented** (opt-in `AIP_REGION`); there is still
no second data plane. `orgs.region` is no longer only a label when the guard is
enabled.

## 1. Target topology

```
global control plane (orgs, routing metadata, billing, audit ids)
  ├─ region eu-central data plane: Postgres + S3 + workers + API
  ├─ region us-east data plane:     Postgres + S3 + workers + API
  └─ region ap-southeast data plane: ...
```

- An org is pinned to exactly one region at creation; the routing table is
  `org_region(org_id, region, data_plane_url, status)`.
- Requests carry the org; the API resolves the org's data plane and **rejects**
  cross-region access instead of proxying.
- Postgres, object storage, audit witness and preview environments stay in the
  org region.
- The global control plane stores IDs and routing only, not tenant content.

## 2. Schema sketch (not migrated)

```sql
CREATE TABLE data_planes (
  region       text PRIMARY KEY,
  api_url      text NOT NULL,
  status       text NOT NULL,  -- active | draining | disabled
  created_at   timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE orgs ADD COLUMN data_plane_region text REFERENCES data_planes(region);
```

## 3. Request flow

1. Auth token contains the org id (already does).
2. Middleware resolves the org's region and compares it with the API's
   `AIP_REGION`.
3. Mismatch → 421/404 with a clear error; never transparently proxy tenant data.
4. Audit writes stay in the region; the global plane can receive only IDs/hashes.

## 4. Migration between regions

- explicit export/import from the backup/restore drill, with a maintenance window
- a new S3 witness is written after the import; the old region is kept read-only
  for the rollback window
- no silent copy; no dual writes

## 5. Acceptance tests before claiming multi-region

1. An EU org cannot read/write through a US API (explicit rejection).
2. Tokens minted in one region are rejected by another unless routed.
3. Audit/witness objects are in the org region.
4. Region migration rehearsed with real data and a rollback.
5. Control-plane loss does not corrupt regional data.
6. Cross-region latency/error metrics are visible.

## 6. What we have

- `orgs.region` label + Workspace UI, audited
- opt-in routing guard: with `AIP_REGION` set, `/api/v1/orgs/{org}/...` returns
  **421 wrong_region** when the org's region differs; live drill: `us-east` → 421,
  `eu-central` → 200 on the Acme org
- no second data plane, no regional storage, no global routing table
- KMS/BYOK is per org, but the KMS endpoint itself is not region-pinned
