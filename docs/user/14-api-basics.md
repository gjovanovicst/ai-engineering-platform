# API basics

The platform exposes a REST API for automation, integrations and reporting. The full endpoint reference is generated from the OpenAPI contract.

## Base URL

| Environment | URL |
|---|---|
| Local | `http://localhost:18080/api/v1` |
| Staging / production | your deployment URL + `/api/v1` |

Every path except `/auth/*` and `/healthz` is scoped to an organisation.

## Authentication

Obtain an access token, then send it as a bearer token:

```bash
TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@company.com","password":"..."}' | jq -r .access_token)

curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:18080/api/v1/orgs/$ORG/projects
```

Access tokens are short-lived. Refresh tokens rotate; store them in an HTTP-only cookie or a secret manager.

## Conventions

- **JSON** request and response bodies.
- **Pagination** with `limit` and `cursor`; the response includes `next_cursor` when more data exists.
- **Idempotency** with an `Idempotency-Key` header on mutating requests; replaying the same key and body returns the original result.
- **Errors** use a stable shape: `{"error":{"code":"...","message":"...","request_id":"..."}}` plus an HTTP status.
- **Rate limits** return `429` with `Retry-After`; honour it.
- **Versioning** is in the path (`/api/v1`); breaking changes require a new version.

## Common status codes

| Code | Meaning |
|---|---|
| 200 / 201 | Success / created |
| 202 | Accepted; a run or async job started |
| 204 | No content |
| 400 | Validation error |
| 401 | Missing or invalid token |
| 403 | Role or project access denied |
| 404 | Resource not found in this organisation |
| 409 | Invalid transition or idempotency conflict |
| 421 | Region mismatch |
| 429 | Rate limited |
| 500 | Server error with a request ID |

## Live events

Run events are available over server-sent events:

```text
GET /api/v1/orgs/{org}/runs/{run}/events
Accept: text/event-stream
Last-Event-ID: 42
```

Reconnect with the last sequence number. The event catalog describes event types and payloads.

## Webhooks

Project and workspace webhooks deliver signed events for run completion, PR state and approvals. Verify the signature before processing; retries use exponential backoff.

## Next

- [API reference](../api.html) — all endpoints and schemas
- [SSE event catalog](api-events.html)
- [Security and audit](security-audit.html)
