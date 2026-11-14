# 15 · Threat model

Scope: the platform as deployed today (API, worker, sandbox, web). The goal is
not a compliance document; it is a list of what can go wrong and what already
stops it.

## 1. Assets

| Asset | Why it matters |
|---|---|
| Source code and diffs | customer IP; the agent writes to it |
| Git tokens (`GITHUB_TOKEN`, `GITLAB_TOKEN`) | can read/write every connected repo |
| JWT signing key (`jwt.pem`) | forges any session |
| Refresh tokens | 30-day session theft |
| Database credentials (`aip_app`, `aip_migrator`) | all tenant data |
| Invite tokens | join a workspace |
| Webhook secrets | forge merge events |
| Model prompts and agent output | may contain secrets from the repo |
| Audit log | non-repudiation |

## 2. Trust boundaries

```
browser ──TLS──> API ──SQL──> Postgres
                   │
                   ├──queue──> worker ──docker/k8s──> sandbox (no network)
                   │                        │
                   │                        └──exported tar──> host git push ──TLS──> GitHub/GitLab
                   └──TLS──> OIDC provider / mail
```

Rules that hold at every boundary:

- the API never executes model output;
- the sandbox never holds a long-lived secret (no git token, no DB DSN);
- git credentials live only in the worker/API process, never in prompts;
- every query is scoped by `org_id` and RLS is the second line.

## 3. STRIDE summary

| Threat | Where | Control | Residual risk |
|---|---|---|---|
| **S**poofing | login, OIDC, webhooks | argon2id + rotation; OIDC state cookie; HMAC / shared token | a leaked refresh token is valid until revoked |
| **T**ampering | run events, diffs, approvals | append-only events; diffs hashed (`diff_sha256`); approvals single-use | DB admin can rewrite rows; no WORM storage |
| **R**epudiation | who approved what | `approvals` records user + time; `audit_log` append-only | no external timestamping |
| **I**nformation disclosure | cross-tenant reads; logs; spans | RLS + `WithOrg`; no prompt/secrets in logs; OTel attributes are ids only | a bug in a new query could leak (RLS catches most) |
| **D**enial of service | login, invite accept, webhooks, runs | per-IP/email rate limits; per-org 600 req/min with `Retry-After`; max 5 active runs per org; body caps (1 MiB); sandbox CPU/mem/PID limits; job deadline | limits are per API replica |
| **E**levation of privilege | role checks, sandbox escape | role matrix per endpoint; non-root container, dropped caps, no network, tmpfs, Job limits | kernel/runtime escape is the container's responsibility |

## 4. Prompt injection

The agent reads repository content and task text. Untrusted text can try to make
it exfiltrate data or run destructive commands.

| Vector | Control |
|---|---|
| Task title/description/AC | treated as data in the prompt, framed as "task briefing" |
| Repository files | agent may read them; it has no secrets and no network |
| Tool output (test logs, grep) | stays inside the sandbox |
| Git host metadata (PR body) | written by us, not fed back into a run |
| Model output → shell | never executed by the platform; only the trusted `AIP_AGENT_COMMAND` runs |

Known gap: the agent command itself is trusted configuration. If an operator
sets it to `sh -c "$PROMPT"`, that operator has broken the boundary.

## 5. Sandbox escape and lateral movement

- Docker driver: `--network none`, non-root image (uid 10001), read-only tmpfs
  for `/tmp`, PID/memory/CPU limits, `/workspace` only, no Docker socket inside.
- Kubernetes driver: one Job, `runAsNonRoot`, dropped capabilities, NetworkPolicy
  denying ingress and egress, active deadline, no retries.
- The workspace is exported as a tar from the container and pushed by the host,
  so the sandbox never holds the git token.

Residual: a container-runtime or kernel escape is out of scope; host hardening
and patching are the operator's job.

## 6. Webhooks and invites (public endpoints)

| Endpoint | Auth | Notes |
|---|---|---|
| `POST /webhooks/github/{repo}` | `X-Hub-Signature-256` HMAC per repo, constant-time compare | body capped at 1 MiB; unknown repo → 404; delivery ids deduplicated for 10 min |
| `POST /webhooks/gitlab/{repo}` | `X-Gitlab-Token` constant-time compare | same; `X-Gitlab-Event-UUID` deduplicated |
| `POST /invites/accept` | 32-byte token (sha256 in DB, 7 days, single use) | creates a user; email comes from the invite, not the request |
| `GET /auth/oidc/{provider}/start` | public | sets an HttpOnly state cookie |
| `GET /auth/oidc/{provider}/callback` | state cookie must match | tokens are delivered in the URL fragment |

Merge is not enough: the PR/MR must also be approved in the platform
(`ApprovedForPR`), otherwise the webhook returns `approval_required`.

## 7. Data handling

- Access tokens live in memory only; refresh tokens are hashed at rest.
- Invite and verification tokens are stored as sha256, never plaintext.
- Logs: no tokens, no prompt text, no email bodies. Errors are wrapped, not
  serialized with payloads.
- Traces: run/org/agent ids and status only.
- Backups must be encrypted at rest (operator responsibility).

## 8. Residual risks accepted for the MVP

1. **No CSRF protection on cookie flows** — the API is bearer-only today; the
   BFF cookie work (E2-S3) must add double-submit CSRF (see `docs/12-auth.md`).
2. **One git token per provider** — any run can push to any connected repo of
   that provider. Per-repo tokens are the next step.
3. **`audit_log` is append-only by convention** (no DB-level revoke yet).
4. **Invite email enumeration** — registration returns 202 for existing emails,
   invite acceptance returns 409; acceptable, revisit with the security review.
5. **Webhook replay dedupe is in-memory per replica** — a replay against another
   replica within 10 minutes is still processed; checks/merges are idempotent.
6. **`golang.org/x/crypto/openpgp` is unmaintained** (GO-2026-5932, module-level);
   the package is not imported or called by the platform.

## 8a. Known repository history issue

An early development `jwt.pem` was committed before `.gitignore` covered it. It
has been removed from the tree, but it remains in history. **It must never be
used anywhere.** Every environment generates its own key with `make keygen`, and
a pilot must rotate the key if it was ever copied from this repository.

## 9. Review checklist (before a pilot)

- [ ] rotate `jwt.pem`, git tokens, webhook secrets after the pilot setup
- [ ] confirm TLS everywhere and `Secure` cookies on the BFF
- [ ] run `make check` and `make e2e` on the exact release commit
- [ ] verify Postgres roles: `aip_app` has NOBYPASSRLS
- [ ] confirm the sandbox image digest is pinned
- [ ] enable `AIP_OTEL_ENDPOINT` and check that no prompt text appears in spans
- [ ] review the last 7 days of `audit_log` for `auth.login.oidc` anomalies

## 10. Security review — 2026-10-24 (E10-S5)

Internal review against the `docs/07` §7 checklist; no external pentest yet.

| Check | Evidence |
|---|---|
| Sandbox flags | Docker driver now uses `--cap-drop ALL`, `--security-opt no-new-privileges`, `--read-only`, tmpfs `/workspace` + `/tmp` + HOME with uid 10001; verified inside a container (`id`, `git init`, commit, `go version`) |
| Egress | sandbox runs with `--network none` unless the spec allows it; agent tool loop cannot reach the internet |
| Secrets | provider keys AES-256-GCM; API returns `has_key`/`key_hint` only; invite/verification tokens sha256 |
| Prompt injection | tool allowlist and read-only guard are enforced server-side; model output is never executed |
| RLS | `make test-integration` (tenant isolation tests) green |
| Rate limit | per-org 600 req/min and 5 active runs: `429` + `Retry-After` (unit + k6 run) |
| Webhooks | HMAC/token constant-time compare, 1 MiB cap, delivery-id dedupe (10 min) |
| Audit | `audit_log` rows for auth, approvals, roles, projects; append-only by rule |
| Dependencies | `govulncheck ./...`: 0 code vulnerabilities (toolchain 1.27.3, spdystream 0.5.1); `pnpm audit --prod`: no known vulnerabilities (postcss >= 8.5.23 override) |
| Logs | structured slog fields are ids/status only; prompts and tokens are not logged at INFO |
