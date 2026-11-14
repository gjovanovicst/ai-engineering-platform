# 04 · API

REST, JSON, `application/json`. The source of truth is [`openapi.yaml`](../api/openapi.yaml);
this document explains the **conventions** that are not visible from the schema.

## 1. Base URL and versioning

```
https://api.{domain}/api/v1
```

- The version is in the path. A breaking change = `/api/v2` + a coexistence period of ≥ 6 months.
- Additive changes (new field, new endpoint) do not change the version. Clients must
  ignore unknown fields.
- `Deprecation` and `Sunset` headers on deprecated endpoints.

## 2. Authentication

| Method | Header | Used by |
|---|---|---|
| Session | `Authorization: Bearer <access_jwt>` | web |
| API key | `Authorization: Bearer aip_<key>` | CI, integrations |
| Refresh | `POST /auth/refresh` with `{refresh_token}` | web |

- Access JWT: 15 min, `EdDSA`, claims: `sub`, `org`, `role`, `exp`, `sid`.
- Refresh: 30 days, rotating; reuse of an old token revokes all sessions.
- An API key carries `scopes` (`tasks:read`, `runs:write`, …) and **has no** access to
  `billing:*` or `org:admin`.

## 3. Tenancy

The org is in the path: `/orgs/{org}/...`. `{org}` is a slug or a UUID. The user must be a
member; otherwise `404` (not `403` — we do not disclose that the resource exists).

## 4. RBAC

| Action | owner | admin | member | viewer |
|---|:--:|:--:|:--:|:--:|
| read projects/tasks/runs | ✓ | ✓ | ✓ | ✓ |
| create/edit a task | ✓ | ✓ | ✓ | — |
| start a run | ✓ | ✓ | ✓ | — |
| plan approval | ✓ | ✓ | ✓ | — |
| PR approval | ✓ | ✓ | ✓ | — |
| prod deploy approval | ✓ | ✓ | — | — |
| manage members | ✓ | ✓ | — | — |
| providers, secrets, audit | ✓ | ✓ | — | — |
| delete org, billing | ✓ | — | — | — |

Checks: `requireRole(min)` middleware + an explicit check in the use case for
`deploy_prod` (two pairs of eyes: two different `user_id`, both `admin`+).

## 5. Pagination, filtering, sorting

```
GET /orgs/{org}/projects/{project}/tasks?limit=50&cursor=eyJjIjoiMjAyNi0xMC0yMVQxMDowMDowMFoiLCJpIjoiLi4uIn0&status=ready&sort=-updated_at
```

- `limit` 1–100 (default 50). `cursor` is opaque base64 (`{created_at, id}`).
- Response: `{ "data": [...], "next_cursor": "..." | null }`; `next_cursor` is `null`
  when there is nothing more. There is no `total` (expensive); a count uses `GET .../count`.
- Filters are plain query parameters; an unknown filter → `400 unknown_filter`.
- `sort` accepts `field` and `-field`; allowlist per resource.

## 6. Idempotency

Required for every `POST` that has an effect (runs, deployments, approvals):

```
Idempotency-Key: 9f1c2b7e-...
```

- The key is stored for 24h together with `request_hash`. Same key + same body → same
  response (`200`, not `201`). Same key + different body → `422 idempotency_key_reuse`.
- No key on these endpoints → `400 idempotency_key_required`.

## 7. Errors (RFC 9457)

```json
{
  "type": "https://api.aip.dev/errors/invalid-transition",
  "title": "Invalid state transition",
  "status": 409,
  "detail": "task T-88 cannot move from review to planning",
  "instance": "/orgs/acme/tasks/65f.../transitions",
  "code": "invalid_transition",
  "fields": [{"path": "to", "message": "allowed: done, blocked"}]
}
```

| HTTP | `code` | Meaning |
|---|---|---|
| 400 | `validation_failed` | body/params do not pass the schema |
| 401 | `unauthenticated` | missing/expired token |
| 403 | `forbidden` / `tool_not_permitted` | RBAC / agent tool |
| 404 | `not_found` | no such resource **or** no access |
| 409 | `invalid_transition`, `conflict` | the state does not allow it |
| 422 | `idempotency_key_reuse`, `budget_invalid` | semantic problem |
| 429 | `rate_limited`, `concurrency_limit` | return `Retry-After` |
| 500 | `internal` | no internal error details in `detail` |

## 8. Rate limiting

- Per user: 600 req/min; per API key: 300 req/min; per org: 3000 req/min.
- `POST /runs`: 30/min per org + `concurrency_limit` (see `orgs.max_concurrent_runs`).
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`,
  `Retry-After` on `429`.

## 9. Endpoint map

| Method | Path | Description | Idempotency |
|---|---|---|---|
| POST | `/auth/login` | email+password → tokens | — |
| POST | `/auth/refresh` | refresh token rotation | — |
| GET | `/auth/me` | profile + orgs + roles | — |
| POST | `/auth/register` | self-serve account + org (owner) | ✓ |
| POST | `/auth/verify-email` | email verification (24h token) | — |
| POST | `/auth/logout` | session revocation + cookie deletion | — |
| POST | `/auth/password/reset-request` | always `202`, no enumeration | — |
| POST | `/auth/password/reset` | new password + revoke all sessions | — |
| GET | `/auth/invites/{token}` | invite overview | — |
| POST | `/auth/invites/{token}/accept` | join the org + tokens | ✓ |
| GET | `/auth/sessions` | active sessions (devices) | — |
| DELETE | `/auth/sessions/{id}` | revoke a single session | — |
| POST | `/auth/switch-org` | new access token for another org | — |
| GET | `/auth/oidc/{provider}/start` | OIDC start (state + PKCE) | — |
| GET | `/auth/oidc/{provider}/callback` | OIDC callback → cookies | — |
| GET/POST | `/orgs` | list / create | ✓ |
| GET/PATCH | `/orgs/{org}` | detail / edit | — |
| GET/POST | `/orgs/{org}/members` | members / invite | ✓ |
| DELETE | `/orgs/{org}/members/{user}` | removal | — |
| GET/POST | `/orgs/{org}/providers` | AI providers (secret write-only) | ✓ |
| GET/POST | `/orgs/{org}/projects` | list / create | ✓ |
| GET/PATCH/DELETE | `/orgs/{org}/projects/{project}` | CRUD | — |
| GET/POST | `.../projects/{project}/tasks` | list / create | ✓ |
| GET/PATCH | `.../tasks/{task}` | detail / edit | — |
| POST | `.../tasks/{task}/transitions` | status change | ✓ |
| POST | `.../tasks/{task}/runs` | start the agent | ✓ |
| GET | `/orgs/{org}/runs` | run list (filter `status`, `project`, `agent_type`) | — |
| GET | `/orgs/{org}/runs/{run}` | detail + phases + usage | — |
| POST | `/orgs/{org}/runs/{run}/cancel` | cooperative cancel | ✓ |
| GET | `/orgs/{org}/runs/{run}/events` | **SSE** stream | — |
| GET | `/orgs/{org}/runs/{run}/diff` | unified diff (artifact) | — |
| GET | `/orgs/{org}/approvals` | pending approvals | — |
| POST | `/orgs/{org}/approvals/{id}/decision` | approve/reject/changes | ✓ |
| GET/PUT | `/orgs/{org}/rules` | rules (per level) | — |
| POST | `/orgs/{org}/rules/{rule}/publish` | new version | ✓ |
| GET/POST | `/orgs/{org}/blueprints` | blueprint list / creation | ✓ |
| POST | `.../blueprints/{bp}/versions` | new version | ✓ |
| GET/POST | `.../projects/{project}/repos` | git link | ✓ |
| GET | `/orgs/{org}/repos/{repo}/pull-requests` | PR list | — |
| GET | `/orgs/{org}/pull-requests/{pr}` | PR detail: checks, files, review findings | — |
| POST | `/orgs/{org}/pull-requests/{pr}/merge` | merge (human only; GitHub merge API) | ✓ |
| POST | `/orgs/{org}/pull-requests/{pr}/findings/{finding}/waive` | waive a blocker finding (admin + reason) | ✓ |
| GET/POST | `/orgs/{org}/design-systems` | design system registry | ✓ |
| POST | `/orgs/{org}/design-systems/{ds}/versions` | new token version (publish) | ✓ |
| GET/POST | `/orgs/{org}/components` | component registry | ✓ |
| POST | `/orgs/{org}/components/{component}/versions` | new component version | ✓ |
| GET/POST | `.../tasks/{task}/comments` | comments on the task | ✓ |
| GET | `/orgs/{org}/notifications` | user notifications | — |
| POST | `/orgs/{org}/notifications/read` | mark as read | ✓ |
| GET | `/orgs/{org}/search?q=` | ⌘K search (FTS) | — |
| GET | `/orgs/{org}/activity` | activity feed (dashboard) | — |
| GET | `/orgs/{org}/deployments/{deployment}/logs` | deploy log (stream/NDJSON) | — |
| GET | `.../projects/{project}/tests` | test results | — |
| GET/POST | `.../projects/{project}/environments` | environments | ✓ |
| POST | `.../environments/{env}/deployments` | deploy | ✓ |
| GET | `/orgs/{org}/analytics/summary` | KPI (with `source`) | — |
| GET | `/orgs/{org}/usage` | usage per project/model | — |
| GET | `/orgs/{org}/audit` | audit log (filter `actor`, `action`, `from/to`) | — |
| POST | `/webhooks/github` | GitHub App webhook (HMAC) | — |
| GET | `/healthz`, `/readyz`, `/metrics` | infra | — |

## 10. Examples

**Starting a run**

```http
POST /api/v1/orgs/acme/tasks/T-88/runs
Idempotency-Key: 9f1c2b7e-1111-2222-3333-444455556666
Content-Type: application/json

{
  "agent_type": "development",
  "model": "gpt-4o",
  "dry_run": false,
  "budget": { "tokens_in": 400000, "tokens_out": 120000, "cost_micros": 5000000, "wall_clock_s": 2400 }
}
```

```http
202 Accepted
Location: /api/v1/orgs/acme/runs/7c9e.../events

{ "id": "7c9e...", "status": "queued", "agent_type": "development", "created_at": "2026-10-21T18:04:11Z" }
```

**Approval decision**

```http
POST /api/v1/orgs/acme/approvals/2b1f.../decision
{ "decision": "approved", "reason": "Plan covers the AC; tests included." }
```

**Changing task status**

```http
POST /api/v1/orgs/acme/tasks/T-88/transitions
{ "to": "ready", "reason": null }
```
A disallowed transition → `409 invalid_transition` with the list of allowed values in `fields`.

## 11. Webhooks (outgoing, `webhook_endpoints` — phase 2)

`run.completed`, `run.failed`, `approval.requested`, `deployment.succeeded`.
Signature `X-AIP-Signature: t=<ts>,v1=<hmac_sha256>`; 5 min tolerance; retry 5×
exponential. In the MVP only the GitHub webhook (incoming); outgoing ones are phase 2.
