# Roadmap — "later" (P2)

> This document exists so that the agents building the MVP **do not close the door** on
> things we will probably build later. Deferred ≠ forgotten: every item has a **trigger**
> (when it is worked on), a **condition** (what must exist) and a **seam** (what must hold
> in the MVP so that later it is possible without a refactor).

## 0. Rules for the agent

1. **Do not implement a P2 item** until it has moved into the active plan (§7). If a story in
   `05-mvp-plan.md` depends on a P2 decision — ask, do not improvise.
2. **Do not close a seam.** If a change in the MVP touches something from §2 (the seam table),
   check whether it is still compatible with the future client. If it is not — that is an ADR,
   not a commit.
3. **Do not build "mobile-only" or "web-only" endpoints.** The API is one; differences go
   through parameters and capabilities, not through duplicated routes.
4. **P2 epics are tracked here**, not in `05-mvp-plan.md`. The MVP plan stays frozen.

## 1. What P2 is (and why exactly that)

| # | Item | Trigger (when) | Why then, and not now |
|---|---|---|---|
| P2-1 | **Native mobile app** | ≥ 30% of active users open the app on a phone; or an explicit need for push/offline | desktop is the primary use case (plan/PR approval, diff review) |
| P2-2 | **PWA** (installable + offline shell) | before native, if "web but like an app" is enough | cheap on top of the same code; does not solve push on iOS without APNs |
| P2-3 | Bitbucket / Gitea | ≥ 3 users ask for it | GitHub and GitLab cover the MVP through the same `git.Provider` port |
| P2-4 | Outgoing webhooks | first third-party integration (Slack, Jira) | the MVP has only the incoming GitHub webhook; the `webhook_endpoints` table already exists |
| P2-5 | Billing (plans, invoices) | first paying customer | the MVP measures cost (`usage_ledger`), it does not charge |
| P2-6 | Plugin SDK | when 3+ teams write their own tools | requires a stable tool protocol (we have it in [11-agent-runtime.md](11-agent-runtime.md)) |
| P2-7 | i18n (en/sr) | first user outside the Serbian-speaking area | all strings must go through `t()` from the start (seam S8) |
| P2-8 | Warm sandbox pool | p95 sandbox start > 8 s or > 100 runs/h | MVP: a cold start of 2–4 s is acceptable |
| P2-9 | Multi-region | data residency requirement | MVP: one region, but `org_id` and S3 keys are region-agnostic |
| P2-10 | Blueprint marketplace | ≥ 10 orgs with their own blueprints | requires versioning and review (we have `blueprint_versions`) |

## 2. Seams the MVP must preserve

This is **required** for all stories in M0–M5. If a story violates a seam — stop and write an ADR.

| # | Seam | Requirement in the MVP | Why (which P2 item) |
|---|---|---|---|
| S1 | Auth for native | `Authorization: Bearer` works without cookies; refresh rotation; `sessions.platform` column (`web`/`mobile`/`cli`) | P2-1 (native does not use an httpOnly cookie) |
| S2 | OIDC PKCE | besides the `client_secret` flow, provide for `code_challenge`/`code_verifier` in the `oidc` adapter | P2-1, P2-2 |
| S3 | Idempotency | every `POST` with an effect accepts `Idempotency-Key` and returns the same response | P2-1 (mobile retry after a network interruption) |
| S4 | Keyset pagination | `?limit=&cursor=`, opaque cursor, no `offset` | P2-1/P2-2 (offline sync and infinite scroll) |
| S5 | State without SSE | `GET /runs/{id}` returns the **complete** state (phases, spent, last event seq) — polling must be enough | P2-1 (background/SSE interruption) |
| S6 | Stable routes | web routes `/{org}/agents/{runId}`, `/{org}/tasks/{ref}` are the contract for deep links | P2-1 (universal links) |
| S7 | Tokens as data | `web/src/design/tokens.json` is the source; the RN theme is generated from it, not rewritten | P2-1, P2-2 |
| S8 | No hardcoded strings | all UI messages through `t('key')` (even if for the MVP it is only the `sr` catalog) | P2-7 |
| S9 | Channel-agnostic notifications | `notifications.kind` + `target`; delivery (in-app/email/push) is separate | P2-1 (push), P2-4 |
| S10 | Artifacts on S3 | diff/log/report always through the `ArtifactStore` port, never on local disk | P2-1 (mobile does not download the repo) |
| S11 | API version in the path | `/api/v1`; breaking change = `/v2` + 6 months of coexistence | all clients |
| S12 | No GraphQL | REST remains the only contract | avoids a double layer and a double client |

## 3. P2-1 · Native mobile app

### 3.1. Assumed use case (not "everything like on the web")

1. **Approval** — plan, PR, prod deploy (the biggest value: approve from a phone).
2. **Monitoring** — active runs, status, cost, "run failed" notification.
3. **Quick review** — diff summary, test result, comments.
4. **Not in the mobile MVP:** blueprint creation, wizard, settings, deep analytics.

### 3.2. Technology (proposal, confirmed by an ADR)

| Layer | Choice | Why |
|---|---|---|
| Framework | **Expo (React Native)** + TypeScript | shared types with `openapi-typescript`, one language with the web |
| Navigation | Expo Router | file-based, the same mental model as the Next App Router |
| Data | TanStack Query + the same generated client | identical cache/retry patterns |
| Auth | `expo-auth-session` + PKCE (S2) | no client secret on the device |
| Push | Expo Notifications → APNs/FCM | we do not write native code for push |
| Theme | `tokens.json` → RN theme object (S7) | visual consistency |
| Offline | TanStack persist + keyset sync (S4) | a read-only cache is enough for the MVP mobile |

**Alternative before native:** PWA (P2-2) — the same code, `manifest.json`, a service worker for
the shell; push on iOS still requires APNs, so a PWA does not replace native if push is critical.

### 3.3. What must be added when P2-1 is activated

| # | Job | Where |
|---|---|---|
| 1 | ADR-0007 "Mobile client" (framework, auth, offline scope) | `docs/adr/` |
| 2 | `push_tokens` table (`user_id`, `platform`, `token`, `last_seen_at`) + registration | `03-data-model.md` |
| 3 | `POST /auth/device` (PKCE exchange) + `sessions.platform='mobile'` | `04-api.md` |
| 4 | Endpoints for approve/monitor (existing, no changes — that is the point of S1–S6) | — |
| 5 | Universal links: `.well-known/apple-app-site-association`, `assetlinks.json` | infra |
| 6 | Push worker (`internal/app/notify/push.go`) — consumes `notifications` | `02-architecture.md` |
| 7 | E2E: Detox/Maestro for 3 flows (sign-in, approve plan, view run) | `07-testing.md` |
| 8 | Store metadata, signing, CI lane (`eas build`) | infra |

### 3.4. What NOT to do now

- ❌ Do not add platform-specific endpoints (`/mobile/...`).
- ❌ Do not introduce a push SDK or the `push_tokens` table before the ADR (seam S9 is enough).
- ❌ Do not build RN components "in advance" — the design system is Tailwind; the RN theme is generated.
- ❌ Do not lock the API to cookie auth (that would kill S1).

## 4. P2-2 · PWA (if it comes before native)

- `manifest.json` (name, icons, `display: standalone`, `theme_color` = `--bg-canvas`).
- Service worker: shell cache + offline fallback page; do **not** cache API responses
  (except `GET /runs/{id}` for the last known state).
- iOS: no `beforeinstallprompt`; the "Add to Home Screen" instruction.
- Seam: S7 (tokens), S4 (keysync), S5 (polling).

## 5. Other P2 items — what "ready" means

| Item | Readiness condition | First step |
|---|---|---|
| P2-3 Bitbucket/Gitea | `git.Provider` port already has two adapters | ADR + adapter |
| P2-4 outgoing webhooks | `webhook_endpoints` + delivery table + retry worker | migration + worker |
| P2-5 billing | plans in `orgs.plan`, metering from `usage_ledger` | ADR (Stripe?) |
| P2-6 plugin SDK | tool protocol stable ≥ 1 month without a breaking change | ADR + sandbox for third-party tools |
| P2-8 warm pool | p95 sandbox > 8 s | worker pre-pull + pool of 2 images |
| P2-9 multi-region | data residency requirement | ADR + replication |

## 6. How this affects current stories

- `E1-S5` (auth) **must** leave `sessions.platform` and a PKCE hook (S1, S2).
- `E3-S10` (SSE) **must** ensure that `GET /runs/{id}` is enough for polling (S5).
- `E2-S1` (shell) **must** use `tokens.json` and `t()` (S7, S8).
- `E9-S3` (notifications) **must** separate `kind` from the delivery channel (S9).
- No story **may** add a "mobile" variant of an endpoint (S12, §3.4).

## 7. Moving P2 → active plan

An item is ready for work when **all 5** exist:

1. An ADR with the decision (framework/scope/risk).
2. An epic with stories and acceptance criteria (the same format as `05-mvp-plan.md`).
3. A cost estimate (time + infra + store).
4. A defined success metric (e.g. "% of approvals from mobile").
5. Confirmation that the seams from §2 are respected in the code at that time (grep + test).

Until then: the item stays here, with the status `P2 — waiting for trigger`.
