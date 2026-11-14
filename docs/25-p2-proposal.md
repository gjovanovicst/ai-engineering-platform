# 25 · P2 proposals (not part of M0–M5)

These are **new stories**, proposed after the M0–M5 plan was completed on
2026-10-25. They are not in `docs/05-mvp-plan.md`; each needs the normal DoR and
review before work starts.

| ID | Story | AC | Size | Status |
|---|---|---|---|---|
| E11-S1 | Installable PWA | manifest + icons; a production service worker caches the shell; `/offline` works; dev is unaffected | S | **implemented 2026-10-25** |
| E11-S2 | Bitbucket Cloud + Gitea adapters | clone/push, PR/MR create+merge, webhook verification for both; provider picker; RLS-safe tokens | M | **done 2026-10-25** (Gitea + Bitbucket) |
| E11-S3 | Billing | plans, seats, usage-based invoice preview, hard limits; Stripe-compatible webhooks | L | proposed |

Rules that still apply: no provider secrets in the database in plaintext, no
merge without a human approval, migrations forward-only, `make check` green.
