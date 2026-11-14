# Use case — one company, practical use

> The scenario is illustrative (names, times and costs are realistic in order of magnitude, not
> measured), but every step corresponds to a real screen, endpoint and gate from this repo.
> It also serves as the basis for the e2e scenario and the seed data ([07-testing.md §9](07-testing.md#9-test-data)).

## 1. The company

**Novasoft** — 18 people, a software agency. Four client projects in parallel
(CRM, portal, HR, billing), 11 developers, 2 QA, 1 tech lead, 1 PM, 1 CTO.

**Problems before the platform:**
- onboarding a new developer onto a project: ~3 weeks (nobody knows where anything is),
- review is a bottleneck (the tech lead spends 2h/day on PRs alone),
- quality varies from project to project (tests "wherever we get to them"),
- the client asks "how much is done", and the answer is an estimate in someone's head.

## 2. Roles in the platform

| Person | Role | What they do in the platform |
|---|---|---|
| Goran (CTO) | `owner` | blueprint, rules, providers, budget, prod approval |
| Marko (tech lead) | `admin` | plan approval, PR review, merge, prod approval (second pair of eyes) |
| Ana, Jovan, 8 others | `member` | tasks, starting runs, reviewing findings |
| Milica (QA) | `member` | test gate, smoke, deploy to staging |
| Petra (PM) | `viewer` | dashboard, analytics, status without going into the code |

## 3. Day 0 — onboarding (2 hours, once)

| # | Step | Screen / command | Takes |
|---|---|---|---|
| 1 | Org + first owner | `aipctl bootstrap --org Novasoft --email goran@novasoft.rs` | 2 min |
| 2 | GitHub App installation (4 repos) | Settings › Git Integrations | 10 min |
| 3 | Provider key (one OpenAI-compatible + Anthropic) | Settings › AI Providers | 5 min |
| 4 | Blueprint `saas-next-go v1` | Templates › New blueprint (5 steps) | 30 min |
| 5 | Design system (tokens from their `tokens.json`) | Design System › Import | 10 min |
| 6 | Org rules v1 (security, testing, style) | Settings › Org Rules → Publish | 15 min |
| 7 | Three projects from the blueprint | Projects › New project | 10 min |
| 8 | Team invites (role per person) | Settings › Members | 5 min |

Result: every project has the same stack, the same test gates, the same rules. A new developer
who joins a project sees the same layout as in every other one — onboarding drops to ~3 days.

## 4. First task — from idea to production (one day)

**Task `T-142`: "SSO login (SAML) for CRM Platform".**

Petra enters it from the client's request:

```
Title: Add SSO sign-in via SAML
AC1: Given a user on /login, When they click "Sign in with SSO", Then they are redirected to the IdP.
AC2: Given the IdP returns a SAML response, When the signature is valid, Then the session is created.
AC3: Given a session exists, When the page is refreshed, Then the user stays signed in.
Out of scope: SCIM provisioning, multi-IdP.
```

### 4.1. DoR and plan (09:12–09:26)

| Time | What | Who | Detail |
|---|---|---|---|
| 09:12 | Task in `ready` | Petra | DoR checklist: 3 AC, module `auth`, scope defined |
| 09:16 | Starting the Planning agent | Marko | model `gpt-4o`, budget $5 / 400k tokens |
| 09:16–09:19 | `planning` run | agent | reads `internal/app/auth`, `web/src/app/(auth)`, 2 test files, org rules v4 |
| 09:19 | Plan reaches approval | — | 5 steps, 6 files, 2 risks (cookie SameSite, IdP timeout) |
| 09:22 | Marko changes step 3 | Marko | adds "E2E test for the callback" |
| 09:26 | **Approve plan** | Marko | goes into `run_events` as `approval.decided` |

The plan (what Marko sees in the drawer):

1. Add SAML configuration in `internal/app/auth` (env + validation)
2. Implement `/auth/saml/login` and `/auth/saml/callback`
3. Add an SSO button on the sign-in screen + redirect handling
4. Write unit tests for signature validation and an E2E test for the callback *(added by Marko)*
5. Update `docs/auth.md` and `.env.example`

### 4.2. Implementation (09:26–09:49)

| Time | Phase | What happens |
|---|---|---|
| 09:26–09:38 | `implement` | 14 tool calls: 6× read, 3× write, 2× search, 1× commit; diff of 6 files, +218/−12 |
| 09:38–09:41 | `test` (1st attempt) | 2 tests fail (`SameSite=None` without `Secure`) |
| 09:41–09:46 | agent fixes | changes the cookie config, re-runs the tests |
| 09:46–09:49 | `test` (2nd attempt) | 184/184 green, coverage +0.4% |
| 09:49 | `review` | 3 findings: 1 major (`RelayState` validation), 2 minor |

**What Marko does:** nothing. He watches the live log if he wants; the run goes on by itself.
The budget is being spent ($0.42 after implementation) and is visible in real time.

### 4.3. Review and PR (09:49–10:20)

| Time | What | Who |
|---|---|---|
| 09:49–10:02 | Ana (dev) resolves the `major` finding | Ana |
| 10:02 | Run finished, diff finalized, PR #142 opened | agent |
| 10:04 | Checks: build ✓, unit ✓, E2E 8/16 running, lint ✓ | CI |
| 10:14 | E2E green (16/16) | CI |
| 10:15 | The "Merge" button becomes enabled | platform |
| 10:20 | Marko reviews the diff (12 min), approve + merge (squash) | Marko |

The PR body automatically contains: plan, test report, coverage delta, link to the run, `Run-Id` trailer.

### 4.4. Deploy (10:20–15:30)

| Time | What | Who |
|---|---|---|
| 10:20 | Auto-deploy to `staging` | agent |
| 10:32 | Smoke tests green | CI |
| 11:00 | Milica checks SSO manually on staging | QA |
| 15:30 | **Prod approval** — Goran + Marko (two different admins) | both |
| 15:34 | Deploy to `prod`, release `v1.42.0` | agent |
| 15:40 | SLO/metrics green, release in the history | system |

### 4.5. Task summary

| Metric | Value |
|---|---|
| From `ready` to `done` | 6h 18m (calendar time, same day) |
| Human time | ~26 min (plan 6, diff review 12, staging smoke 8) |
| Agent cost | **$2.14** (planning $0.31, implement $1.42, review $0.41) |
| Iterations | 2 test cycles, 1 review fix |
| Audit | 47 `run_events`, 14 tool calls, 6 artifacts, 5 audit records |

Before the platform, the same task took: ~5 working days (waiting for review, manual setup, forgotten tests).

## 5. What happens in the background (for this to be possible)

| Mechanism | Where it is visible |
|---|---|
| `rules_snapshot` (org v4, project v2) | Run's Context tab — audit proof of what the agent used |
| Budget before every call | `usage.updated` event; cancel before exceeding it |
| Checkpoint per phase | the run survives a worker restart |
| Sandbox per run | the agent could not touch `prod` or another repo |
| Tool allowlist | `development` must not `http.fetch`; every rejected call is recorded |
| Artifacts with a hash | plan, diff, test report — reproducible |

## 6. When it does not go well — the same task, a different outcome

**Scenario B (test gate does not pass):** the third attempt still fails (external IdP mock).
The run goes to `failed` (`code: test_gate_failed`), the diff is kept as an artifact, the task returns
to `in_progress` with the agent's comment. Marko opens the diff, sees that the problem is in the mock,
fixes it manually and starts a new run for the test phase only. **Nothing is lost** — that is
the point of checkpoints and artifacts.

**Scenario C (review blocks):** the review agent finds a `blocker` (the signature is not verified in
constant time). Merge is blocked. Ana can fix it, or Goran can "waive" the
finding — but only with a reason, and that is an `audit` record with a name. No silent skipping.

## 7. Weekly rhythm

| When | What | Who | Takes |
|---|---|---|---|
| Monday | Grooming: tasks into `ready` (DoR) | Petra + Marko | 30 min |
| Every day | Dashboard review: 3 active runs, 2 approvals, 1 failed | Marko | 10 min |
| Wednesday | Blueprint/rules changes (if the team hits a Pattern) | Goran | 20 min |
| Friday | Client demo + retrospective: what the agent got wrong | everyone | 45 min |
| Monthly | Cost review ($/task per project), blueprint update | Goran | 1h |

## 8. What the company gains (illustrative, first 90 days)

| Metric | Before | After |
|---|---|---|
| Lead time (ready → done) | ~5 days | 1–2 days |
| Review load on the tech lead | 2h/day | 40 min/day |
| Onboarding onto a project | 3 weeks | 3 days |
| Escaped defects (monthly) | 6–8 | 2–3 |
| Cost per task (agent) | — | $1.50–$3.00 |
| "Where are we with the request?" | estimate | dashboard, `measured` |

## 9. Boundaries — what the platform does **not** do

- It does not decide architecture instead of the team; a human approves the plan, and that is the only real control.
- It does not merge or deploy to `prod` without people (two admins for prod).
- It does not write a feature without acceptance criteria — a task without DoR does not enter `planning`.
- It does not guarantee that the code is good; it guarantees that the **process** is the same every time and that everything
  is recorded. Quality still depends on review and on the model.

## 10. How we use this as a test

The same scenario is the e2e test (`web/e2e/sso-flow.spec.ts`) and the seed:

```bash
aipctl seed --demo --scenario sso
# org Novasoft, 3 projects, T-142 in ready, 1 run awaiting_approval, 1 failed run
```

Proof that the documentation is not a lie: if the steps from §4 cannot be carried out in the UI on
the seed data, then either the UI or the documentation is wrong.
