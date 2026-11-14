# Threat model (Phase B)

Status: **documented 2026-10-27** · owner: platform
Scope: the run execution path, credentials, tenant data, delivery.

## Assets

- source code and diffs from tenant repositories
- provider keys, GitHub tokens/app keys, webhook secrets, JWT signing key
- tenant data (tasks, runs, attachments, audit log)
- the ability to push/merge code

## Trust boundaries

1. **Browser → API** (session JWT, org claim)
2. **API → Postgres** (app role, RLS per org)
3. **Worker → sandbox** (Docker, per-run container)
4. **Worker → GitHub/LLM providers** (outbound tokens)
5. **GitHub → webhook** (HMAC signature)
6. **Operator config → commands** (`AIP_AGENT_COMMAND`, `AIP_MCP_SERVERS`)

## Threats and controls

| # | Threat | Control now | Residual |
|---|---|---|---|
| T1 | Agent escapes the sandbox | Docker container, non-root `agent` user, `--cap-drop ALL`, `no-new-privileges`, read-only rootfs, tmpfs workspace/cache, pids/memory limits | no seccomp/AppArmor profile; Docker daemon remains host-root |
| T2 | Agent exfiltrates code/data | agent/run sandbox `NetworkDisabled: true`; MCP per-server sandbox defaults to disabled unless `allow_network` | a server with `allow_network` has unrestricted egress, no domain allowlist |
| T3 | Credentials leak in prompt/logs | provider keys stay in worker memory; secret scanning of diffs; no prompt logging | logs still contain error bodies; review before shipping |
| T4 | Malicious attachment | 5 MiB/MIME allowlist, clamd scan (fail-closed when configured), SHA-256 | clamd is optional; S3 object is not rescanned |
| T5 | Cross-tenant access | RLS on all tenant tables; org claim checked in handlers; aggregate metrics use a counts-only function | DB superuser can bypass RLS |
| T6 | Forged webhook | HMAC SHA-256 per provider; per-org GitHub App secret route | PAT-webhook secret is global |
| T7 | Stolen session | EdDSA access token 15 min, rotating refresh, argon2id passwords, rate limit | no device/session management UI |
| T8 | Supply chain | pinned image digests in prod; `make security` (govulncheck + pnpm audit) runs locally and in the CI `security` job | branch protection must still mark it required |
| T9 | Malicious MCP tool names/output | namespacing prevents shadowing; every MCP tool output is scanned and blocked on a credential finding | scanner is pattern-based and can miss novel formats |
| T10 | Tampered audit log | append-only by application policy | DB admin can rewrite; see docs/32 |

## Verified facts (2026-10-27)

- API container has **no Docker socket**; only the worker mounts it.
- Agent backends create sandboxes with `NetworkDisabled: true`.
- MCP servers get their own sandbox with `NetworkDisabled: !allow_network`.
- RLS is enabled on tenant tables; the metrics function returns counts only.

## Gaps / next

1. seccomp/AppArmor profile for the sandbox
2. domain-level egress allowlist (currently boolean only)
3. mark the CI `security` job as a required check and pin govulncheck
4. per-tool MCP output redaction/scan
5. audit tamper-evidence (docs/32)
