# Changelog

All notable changes to this project. Dates are 2026.

## Unreleased — 2026-11-05

### Added

- **Per-repository git credentials**: an encrypted PAT per repository that takes
  precedence over the GitHub App installation token and the global
  `AIP_GITHUB_TOKEN`; owner/admin-only API and Repositories UI, audit events,
  and a git-providers user guide (`docs/user/15-git-providers.md`).
- **ASDLC flow**: task → plan approval → development → test/design gates → PR →
  CI/human approval → merge → metrics, with SSE run events.
- **Enterprise controls**: project RBAC, append-only audit chain + witness, legal
  hold, SIEM delivery, policy-as-code, approval rules, change windows, merge
  queue, previews, incidents.
- **Identity**: OIDC + SAML SP (signed AuthnRequests, encrypted assertions, SLO),
  SCIM users/groups, per-org SSO settings, API keys.
- **BYOK**: `org_crypto`, per-organisation KMS, v4 envelope, encrypted KMS token,
  Settings UI, revoke drill.
- **Custom agents**: OCI-image registry, digest allowlist, project selection,
  worker execution, Settings UI, cosign verifier, fake-GitLab e2e.
- **HA/ops**: Patroni platform database (etcd + 2 Spilo + HAProxy), failover,
  partition/quorum/replica drills, logical backup, cold volume backup and
  standalone boot restore, gated sync-key recovery.
- **PITR**: durable MinIO WAL archive + base backups (wal-g, 30/30 retention,
  credentials in a 0600 env file) + restore from the base to a timestamp between
  two marker writes (docs/60/63).
- WAL archive metrics and Prometheus alerts for failures/staleness.
- PITR restore request API/UI and a worker-driven restore service; DR dashboard
  with restore success/last-age/duration SLOs and a 30-day stale-restore alert.
- Offsite MinIO mirror with heartbeat and 48h stale alert; restore drill from
  the offsite endpoint succeeded.
- Real Git rebase conflict test for the merge queue (conflict blocks merge).
- GitLab merge-request rebase support with async conflict detection.
- K8s sandbox: read-only root, seccomp, bounded emptyDir volumes and MCP exec.
- K8s live drill on kind: read-only root, writable workspace and MCP stdio passed.
- Unsupported egress allowlist env now fails startup instead of being ignored.
- WAL scripts moved into the Patroni data volume after a recreate stopped archiving.
- Base-backup script/heartbeat, 48h staleness alert and a cron install artifact.
- Vault-dev root-token and AppRole secret sync/rotation PoCs for WAL
  credentials (docs/61/62).
- **Multi-region**: opt-in `AIP_REGION` routing guard (421 wrong_region).
- **UI**: visual baseline-vs-current, structured retry panel, blueprint templates,
  per-project onboarding timer.
- **Success metrics**: first-pass merge, proof validity, dev runs/task and an
  escaped-defect proxy in `/metrics` and Analytics.
- **Concurrency**: ten-task API e2e with ten distinct merged PRs (docs/57).
- **Supply chain**: cosign in the API image, JS license allowlist, govulncheck.

### Changed

- Platform API/worker run on Patroni through `patroni-haproxy`; the old Postgres
  remains a pre-cutover rollback snapshot.
- API/worker use HAProxy → PgBouncer (prepared-statement tracking).
- Per-org worker fairness uses Postgres advisory-lock slots
  (`AIP_ORG_MAX_CONCURRENT_RUNS`).

### Fixed

- Deterministic frozen context: tar metadata/order and agent type no longer change
  the hash (e2e no longer fails `context_changed`).
- `GO-2026-4753`: upgraded goxmldsig to 1.6.0 (SAML signature bypass).
- Typed-nil KMS interface ("kms: not configured").
- e2e no longer runs `docker compose down -v` against the developer stack.
- `.git/` no longer changes the frozen context hash under concurrent pushes.
- `visual/*` artifacts are allowed by the DB check and stored as PNGs

### Verified

- `make check`, `make security`, `make e2e` green.
- Patroni: Leader running, Replica streaming lag 0 MB; API connects through
  Patroni (6 connections observed).
- Data snapshot: 12 orgs / 12 projects / 330 tasks / 374 runs.
- Sandbox: no caps, no mounts, no network, read-only root, writable tmpfs.

### Known gaps

- Host fencing: no `/dev/watchdog`, systemd offline, no cloud STONITH
  (docs/56).
- Sync replication: stale `/service/<scope>/sync`; sync mode stays off, manual
  gated recovery only (docs/53).
- Multi-region: routing guard only; no second data plane.
- Volume restore boots standalone, not as a Patroni node (docs/51).
- Backup/PITR, live embedding provider, seccomp/AppArmor and K8s sandbox are not
  drilled.

See [docs/54-status.md](docs/54-status.md) for the authoritative status and
[docs/28-enterprise-roadmap.md](docs/28-enterprise-roadmap.md) for the plan.

## Open-source preparation

- Apache-2.0 license, NOTICE, security/contribution/governance policies,
  issue/PR templates and `make oss-audit`.
- Public-readiness plan in `docs/OPEN_SOURCE.md`; the repository is not published
  yet and the history still needs a secret scan.
