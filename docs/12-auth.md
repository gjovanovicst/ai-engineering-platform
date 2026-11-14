# 12 · Auth & account lifecycle

The only real gap in the specification before M0. This document defines how an account comes into
being, how it is verified, how a user signs in, and how tokens are stored — precisely enough that
`E1-S5` can be implemented without questions.

## 1. Model

| Entity | Table | Note |
|---|---|---|
| Account | `users` | `email` unique (citext), `password_hash` nullable (OIDC-only) |
| Email verification | `email_verifications` | token hash, 24h, single-use |
| Password reset | `password_resets` | token hash, 1h, single-use |
| Session | `sessions` | refresh hash, `platform`, revocation |
| Invite | `invites` | 7 days, `accepted_at` |
| Sign-in attempts | `login_attempts` | rate limit and lockout |

Additions to `users`: `email_verified_at timestamptz`, `last_login_at timestamptz`.
Additions to `sessions`: `last_used_at timestamptz`, `revoked_reason text`.

## 2. Registration

Two entry points, the same table:

| Path | Who | What is created |
|---|---|---|
| **Self-serve** `POST /auth/register` | new user | `users` + `orgs` (slug from the name) + `memberships(owner)` |
| **Invite** `POST /auth/invites/{token}/accept` | invitee | `users` (if it does not exist) + `memberships(role from the invite)` |

### 2.1. Self-serve flow

```
POST /auth/register { email, password, name, org_name? }
  → 201 { user_id, email_verified: false, org_slug }
  → email with a link /verify?token=...
POST /auth/verify-email { token }
  → 200 { verified: true }            # only now are invite/provider/deploy allowed
```

- Password: min 12 characters, max 128; `zxcvbn` score ≥ 3; reject common passwords and those that
  contain the email; argon2id (`t=3, m=64MiB, p=2`, salt 16B, key 32B, PHC string).
- If the email already exists → **202** (same as success) and a "someone tried to register"
  email is sent; we never reveal the existence of an account.
- `org_name` default = `"{name}'s workspace"`; the slug is deduplicated (`acme`, `acme-2`).

### 2.2. Invite flow

```
POST /orgs/{org}/members { email, role }      → 201, email with a link (7 days)
GET  /auth/invites/{token}                    → 200 { org_name, email, role, expires_at }
POST /auth/invites/{token}/accept { name, password? }
  → 200 TokenPair (+ cookie)
```

- If the email is already registered: accepting only adds a `membership` (no password).
- If not: creates an account with `email_verified_at = now()` (possession of the inbox is proof).
- An invite can be revoked (`DELETE /orgs/{org}/members/invites/{id}`) and is not modified.

## 3. Sign-in

```
POST /auth/login { email, password }
  → 200 { access_token, refresh_token, expires_in: 900 } + Set-Cookie (web)
```

- Verification: rate limit → user lookup (constant-time even for a nonexistent email: dummy argon2
  verify) → password verify → rehash if the parameters are outdated → session.
- Failure: always `401 unauthenticated`, `detail: "Invalid email or password"` (no distinction
  between "no account" / "wrong password").
- After 10 failures for the same email in 15 min → `429` with `Retry-After` (computed from
  `login_attempts`, not from IP alone).
- Success writes `audit_log(action='auth.login.succeeded')`; failure writes
  `auth.login.failed` with `ip` and `user_agent` (no password, of course).
- `email_verified_at IS NULL` → login **works**, but the response carries `"email_verified": false`
  and the UI shows a banner; the following are forbidden: invite, provider create, deploy.

## 4. Tokens

### 4.1. Access JWT

| Field | Value |
|---|---|
| alg | `EdDSA` (Ed25519), key in `AIP_JWT_KEY` (PEM) |
| `sub` | user id |
| `org` | active org (changed via `/auth/switch-org`) |
| `role` | `owner|admin|member|viewer` in that org |
| `sid` | session id (for revocation) |
| `scopes` | `["*"]` for a session; explicit for an API key |
| `iss` / `aud` | `aip` / `aip-api` |
| `exp` | 15 min |

Verification: signature, `exp` (±30s skew), `aud`, `iss`, and **`sid` is not revoked**
(revocation cached in Redis for 60s; without Redis → DB check by `sid`, only for sensitive routes).

### 4.2. Refresh token

- 32 random bytes (base64url), **only `sha256`** in the database.
- Rotated on every `/auth/refresh`: the old one is marked `revoked_at = now()`, a new one is issued.
- **Reuse detection:** if an already rotated token appears → revoke **all** of the user's sessions,
  write `auth.refresh.reuse_detected`, send an email "did you sign in from a new device? …".
- TTL 30 days; absolute limit 90 days (`sessions.created_at`).

### 4.3. Cookies (web BFF)

| Cookie | Content | Attributes |
|---|---|---|
| `aip_rt` | refresh token | `HttpOnly; Secure; SameSite=Lax; Path=/api/v1/auth; Max-Age=2592000` |
| `aip_csrf` | 32B random | `Secure; SameSite=Lax; Path=/` (read by JS) |

- The access token is **not** put in a cookie — it is held in page memory; after a reload
  a silent `POST /auth/refresh` follows (cookie) and new memory.
- `POST /auth/logout` deletes both cookies and revokes the session.

### 4.4. CSRF

State-changing requests authenticated **by cookie** must carry:

```
X-CSRF-Token: <value of the aip_csrf cookie>
Origin: https://app.<domain>        (allowlist)
```

- Double-submit + Origin check; both must pass.
- Requests with `Authorization: Bearer` are exempt (CSRF does not exist without ambient credentials).
- Failure → `403 forbidden`, `code: csrf_failed` (never 401 — so as not to confuse the refresh logic).

## 5. Password reset

```
POST /auth/password/reset-request { email }   → 202 (always, regardless of existence)
POST /auth/password/reset { token, new_password } → 200
```

- Token 1h, single-use; stored as a hash.
- Successful reset: revoke **all** of the user's sessions + `audit_log(auth.password.reset)`.
- Rate limit: 3/h per email, 10/h per IP; the same response when limited? No — `429`
  is fine because it does not reveal the existence of an account.
- The link points to `https://app.<domain>/reset?token=...`; the token is used once.

## 6. OIDC (GitHub, Google)

```
GET /auth/oidc/{provider}/start?redirect_uri=/projects
  → 302 to the provider (state + PKCE S256 + nonce, state in a signed cookie for 10 min)
GET /auth/oidc/{provider}/callback?code&state
  → 302 to redirect_uri + Set-Cookie(aip_rt) + aip_csrf
```

- Mapping: `provider_user_id` is stored in `user_identities(provider, subject, user_id)`.
- If the email exists: **link** (only if `email_verified` at the provider).
- If it does not exist: create an account, `email_verified_at = now()`.
- An OIDC-only account has no password; a password reset does not touch it (the email says "use GitHub").
- Native (P2): the same start with `redirect_uri=aip://auth`, then `POST /auth/oidc/exchange`
  (one-time `code`, 60s) — the endpoint is added together with ADR-0007, not now.

## 7. Email adapter

```go
type Mailer interface {
    Send(ctx context.Context, msg Message) error   // Message: To, Template, Data
}
```

| Environment | Provider |
|---|---|
| dev/test | Mailhog (`localhost:1025`), templates are previewed at `:8025` |
| staging/prod | Postmark (transactional) or SES, `no-reply@<domain>` |

- Templates: `verify_email`, `reset_password`, `invite`, `refresh_reuse_warning`.
- Retry: 3× exponential (1s, 4s, 16s); after that `failed` in `email_outbox` (the table is
  added with the implementation; until then only a log + metric).
- The email contains **only** the link and context; never a password or a token in a log.

## 8. Sessions and devices

- `GET /auth/sessions` → list of active ones (device, IP, `last_used_at`, `platform`).
- `DELETE /auth/sessions/{id}` → revoke one; `POST /auth/sessions/revoke-all`.
- Password change/reset/reuse → revoke all except the current one (optionally "sign out everywhere").
- `platform` (`web|mobile|cli`) is set based on the `X-Client-Platform` header or a
  `User-Agent` fallback; it is used in the UI list and in the audit (seam S1).

## 9. Rate limiting (auth)

| Route | Limit |
|---|---|
| `POST /auth/login` | 5/min per IP+email, 20/h per email |
| `POST /auth/register` | 3/h per IP |
| `POST /auth/password/reset-request` | 3/h per email, 10/h per IP |
| `POST /auth/verify-email` | 5/h per IP |
| `POST /auth/refresh` | 60/min per session |

Implementation: Redis token bucket (key `rl:{route}:{scope}`); without Redis → in-memory
fallback per instance (degradation, not an outage).

## 10. Audit events

`auth.register`, `auth.email.verified`, `auth.login.succeeded`, `auth.login.failed`,
`auth.logout`, `auth.refresh.rotated`, `auth.refresh.reuse_detected`,
`auth.password.reset_requested`, `auth.password.reset`, `auth.invite.accepted`,
`auth.session.revoked`, `auth.oidc.linked`.

Each carries `actor_id` (or `null`), `ip`, `user_agent`, `metadata` (e.g. `provider`).
Never contains a token, a password, or a hash.

## 11. Acceptance criteria for `E1-S5`

- [ ] Registration → verification → login works end-to-end; a nonexistent email on
      `reset-request` returns the same `202`.
- [ ] Refresh rotates the token; reuse of an old one kills all sessions (test).
- [ ] `aip_rt` is `HttpOnly` and is not visible in JS; `aip_csrf` exists.
- [ ] A state-changing request without `X-CSRF-Token` (cookie auth) → `403 csrf_failed`.
- [ ] A Bearer request without a CSRF token passes (CLI).
- [ ] 11 wrong passwords in 15 min → `429` with `Retry-After`.
- [ ] A password reset revokes all sessions.
- [ ] Invite accept with an existing account only adds a membership.
- [ ] All events from §10 in `audit_log`.
- [ ] No log contains a token/password (grep in the test).

## 12. Bootstrap of the first user

See [02-architecture.md §18](02-architecture.md#18-bootstrap-and-seed). In short:

```bash
aipctl bootstrap --org "Acme" --email goran@acme.io --name "Goran"
# → creates org + user (email_verified_at = now()) + membership(owner)
# → prints a one-time password to stdout (never to the log)
```

`bootstrap` is idempotent: if the org or the user exists, it changes nothing and returns `already`.
For demo data there is a separate `aipctl seed --demo` (see [07-testing.md §9](07-testing.md#9-test-data)).


## Invite acceptance (no account yet)

`POST /api/v1/invites/accept` with `{token, name, password}` creates the user
with the invited email, adds the membership and returns a session. The
authenticated variant `POST /orgs/{org}/invites/accept` remains for users who
already have an account.

## Client rule: serialize refresh

Refresh tokens rotate, and presenting an already-rotated token revokes every
session of that user (reuse detection). A client must therefore never send two
refresh requests with the same token: keep a single in-flight refresh promise
and share it between concurrent callers (`apps/web/src/lib/api.ts`). React
StrictMode runs effects twice in development, which makes this a real failure
mode, not a theoretical one.
