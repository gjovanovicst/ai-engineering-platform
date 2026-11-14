# Project status — 2026-11-05

This is the honest snapshot. Only what is listed under **verified** is claimed.

## Product

An AI engineering platform: task → plan approval → development → test/design
gates → PR → CI/human approval → merge → metrics, with project RBAC, audit,
policy, BYOK, SSO/SCIM, HA Postgres and custom agents.

## Verified in this environment

| Area | Evidence |
|---|---|
| checks | `make check` EXIT=0 |
| security | `make security` EXIT=0 (govulncheck, `pnpm audit`, JS license allowlist) |
| GitLab rebase | async rebase API + conflict polling, unit-tested (`docs/70`) |
| merge conflict | real `git rebase` conflict test: PR 2 fails on conflict and is not merged (`docs/69`) |
| e2e | `make e2e` EXIT=0 — plan, approval, diff, MR, gate, merge, metrics; a custom plugin is selected; `E2E_CONCURRENT=1 N=10` merged ten distinct PRs (docs/57) |
| live stack | API `/readyz` 200, web `/projects` 200, worker healthy |
| platform DB | Patroni: Leader running, Replica streaming lag 0 MB; API/worker on `patroni-haproxy` |
| data | 12 orgs / 12 projects / 330 tasks / 374 runs |
| HA drills | failover 5s (direct) / 13s (HAProxy), etcd 3→2→0→3, quorum-loss/partition, replica rejoin |
| backups | logical 12/12/330/374; cold volume backup booted standalone and read the same counts |
| PITR/restore | durable MinIO WAL + base backup; restore request API/UI + runner succeeded (12/12/330); WAL/base + restore SLO health in `/dr`; restore heartbeat metric and 30-day alert; worker-driven restore (docs/66); offsite MinIO mirror + restore from it (docs/67/68); WAL scripts survive recreate (docs/60–68) |
| SAML | SP metadata, SP-initiated login, ACS, signed AuthnRequests, encrypted assertions, SLO and replay cache; `scripts/saml-drill.js` logged in and rendered `/projects` |
| BYOK | org_crypto + v4 envelope, encrypted KMS token, UI, revoke drill 400 |
| custom agents | registry, digest allowlist, cosign verifier, signed 201 / unsigned 400, UI, e2e execution |
| multi-region | opt-in `AIP_REGION` routing guard: mismatch 421, match 200 |
| UI | visual baseline-vs-current, retry panel, blueprint templates |
| success metrics | first-pass merge, proof validity, dev runs/task, escaped-defect proxy (docs/16 §8) |
| egress | `AIP_SANDBOX_EGRESS_ALLOW` is fail-closed; no domain allowlist (docs/73) |
| sandbox | K8s live drill: Job, read-only root, writable workspace, MCP `cat` stdio (`docs/72`); K8s Job hardening + MCP StartProcess implemented and unit-tested (not cluster-drilled, docs/71); Docker hardening drill: no caps/mounts/network, read-only root, writable tmpfs workspace (docs/55) |
| fencing | preflight blocked: no `/dev/watchdog`, systemd offline, no cloud STONITH (docs/56) |

## Not done / blocked

| Item | Blocker |
|---|---|
| multi-region data plane | second cluster, regional object storage/audit, global routing |
| host fencing | `/dev/watchdog` or cloud STONITH |
| sync RPO | stale `/service/aip-ha/sync`; gated manual recovery only, sync off |
| full node rejoin from volume | new Spilo node + DCS; only standalone boot verified |
| live embedding provider test | requires a provider key |
| domain egress allowlist / seccomp | host/container runtime policy |
| per-project onboarding timer | implemented: task/run/PR/merge hours, expandable in Analytics |

## Topology

```
api/worker → HAProxy :6432 → 2× PgBouncer → (old Postgres, rollback snapshot)
api/worker → patroni-haproxy :5432 → Patroni (etcd + 2 Spilo) → current platform DB
web (Next.js dev) → API :18080
```

`.env` currently points `AIP_DB_DSN` at Patroni and `AIP_REGION=eu-central`.
The old Postgres is a pre-cutover snapshot, **not** a replica; rollback discards
writes after the cutover.

## Risks

- no host fencing; a wedged old primary could still be reachable
- async replication only; an unreplicated ack can be lost on failover
- backups: Patroni volume boot was standalone, not a cluster rejoin; no PITR drill
- BYOK KMS token is protected by the deployment key, not an HSM
- plugin cosign key mode used insecure-registry/tlog flags for the local drill
- `AIP_REGION` mismatch rejects all orgs if misconfigured
- demo data includes drill/test rows

## Next decisions

1. host fencing drill (watchdog/STONITH)
2. second data plane / region
3. sync-mode cleanup design with fencing
