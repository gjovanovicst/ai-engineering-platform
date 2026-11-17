# 01 · ASDLC — AI Software Development Lifecycle

> ASDLC is the process that the platform **enforces** and by which the platform **is built**.
> One document, two applications. If these two diverge, the product lies.

## 0. Three invariants

1. **Every transition is explicit.** There is no "the agent just continued". A transition is an event in
   `run_events` with a type, a cause, and an actor (`agent` | `human` | `system`).
2. **Every artifact has a version and a hash.** Plan, diff, test report, rules — everything carries
   `sha256` and `version`. Reproducing a run must not depend on "current state".
3. **A gate cannot be skipped, only explicitly bypassed.** `--force` exists,
   but it writes an `override` event with the user and the reason.

## 1. Phases

```
        ┌──────────┐   ┌───────────┐   ┌──────────┐   ┌────────┐
Idea  → │ INTAKE   │ → │ BLUEPRINT │ → │ BACKLOG  │ → │  DoR   │
        └──────────┘   └───────────┘   └──────────┘   └────┬───┘
                                                           ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │                         AGENT RUN                                │
   │  context → plan → [GATE: plan approval] → implement → test       │
   │         → review → [GATE: PR approval] → merge                   │
   └──────────────────────────────────────────────────────────────────┘
                                    ▼
   ┌────────────┐   ┌───────────┐   ┌─────────┐   ┌──────────┐
   │  RELEASE   │ → │  DEPLOY   │ → │ OBSERVE │ → │  LEARN   │
   │ [GATE:prod]│   │           │   │         │   │          │
   └────────────┘   └───────────┘   └─────────┘   └──────────┘
```

| Phase | Actor | Input | Output (artifact) | Gate |
|---|---|---|---|---|
| **Intake** | human | idea, requirement, bug | `intake` record + module proposal | — |
| **Blueprint** | human + agent | intake | `blueprint_version` (stack, DS, test strategy, rules) | blueprint publish |
| **Backlog** | human + agent | blueprint | `task` with AC | — |
| **DoR** | human | task | task in `ready` status | **DoR** (see §4) |
| **Context** | agent | task + rules + repo | `context_manifest` (files, rules, token budget) | — |
| **Plan** | agent | context | `plan` (steps, files, risks, estimate) | **plan approval** |
| **Implement** | agent | plan | `diff` + commit in the sandbox | — |
| **Test** | agent | diff | `test_report` (pass/fail, coverage delta) | **test gate** (see §5) |
| **Review** | agent + human | diff + test report | `review` (findings, severity) | **PR approval** |
| **Merge** | human (or CI) | approved PR | merge commit | branch protection |
| **Release** | agent | merge | `release` (version, changelog) | — |
| **Deploy** | agent | release | `deployment` | **prod approval** |
| **Observe** | system | deploy | SLOs/metrics | — |
| **Learn** | human | observe + runs | changes to the blueprint/rules | publish |

## 2. Task state machine

```
draft ──▶ ready ──▶ planning ──▶ in_progress ──▶ review ──▶ done
  │        │            │              │             │
  └────────┴────────────┴──────────────┴─────────────┴──▶ blocked
                                                          │
                                                          └──▶ canceled
```

Transitions (allowed; everything else is `409 invalid_transition`):

| From | To | Condition |
|---|---|---|
| `draft` | `ready` | DoR met (§4) |
| `ready` | `planning` | an active run of type `planning` exists |
| `planning` | `in_progress` | plan approved (`approval.type=plan`) |
| `in_progress` | `review` | diff + green tests |
| `review` | `done` | PR approved and merged |
| any | `blocked` | reason required (`blocked_reason`) |
| any | `canceled` | human only |

## 3. Run state machine

```
queued ──▶ running ──▶ awaiting_approval ──▶ running ──▶ succeeded
   │           │                │                            │
   └───────────┴────────────────┴──────────▶ failed | canceled
```

- `awaiting_approval` is the **only** place where a run stops and waits. Time limit
  (default 72h) → `expired` (subtype `failed`).
- Cancel `canceled` is cooperative: the worker sets `cancel_requested_at`, the runtime
  checks between steps and kills the sandbox.
- A worker restart does not lose the run: every phase is a checkpoint in `run_phases` + `run_events`.

## 4. Definition of Ready (DoR)

A task is `ready` only if **all 6** hold:

1. The title is an imperative sentence (`Add SSO login`, not `SSO`).
2. There is ≥ 1 acceptance criterion in Given/When/Then form.
3. The module/area and affected packages are indicated (e.g. `internal/app/task`).
4. What is **not** in scope is defined (anti-AC), if there is a risk of scope creep.
5. Dependencies are listed (other stories, external services) or explicitly "none".
6. Estimate ≤ 1 day of agent work; if larger — the task is split.

A task without a DoR **does not enter** `planning`. This is the most important discipline in the system.

## 5. Gates

### G1 · Plan approval (human)
The agent submits a plan: steps, files, risks, the tests it will write, cost estimate.
The human: `approve` / `request_changes` / `reject`. Without this there is no `implement` phase.

### G2 · Test gate (automatic + threshold)
- all required test bundles green,
- coverage delta ≥ −1% (a drop greater than 1% blocks),
- no `ERROR` in the log matching `panic|fatal`,
- lint/`go vet`/`tsc --noEmit` clean.
The thresholds are in the blueprint (`test_strategy.gates`), not hardcoded.

The development diff is also scanned for credentials before the PR. Deliberate
test fixtures and generated examples must be recognisable: the scanner ignores
placeholder values (`example`, `redacted`, `never-issued`, `dummy`, `fake`, …)
and lines carrying the pragma `aip:allow-secret` on the same line or the added
line above. A real-looking token without the pragma fails the run with
`secret_detected`.

### G3 · PR approval (human + agent review)
The agent review produces findings classified `blocker | major | minor | nit`.
A `blocker` must be resolved or explicitly waived (with a reason). The merge is done by a human or
by a bot with a branch protection rule; the agent **does not have** merge permission.

### G4 · Production deploy (human, two pairs of eyes for `prod`)
A deploy to `prod` requires approval from a user with the `admin` role **and** passing staging
smoke tests. Rollback is one click, but it executes as a new `deployment` record.

## 6. Agent roles

| Agent | Phase | May change code | Typical output |
|---|---|---|---|
| `planning` | plan | no | plan + file list + test plan |
| `development` | implement | yes (in the sandbox) | diff + commit |
| `testing` | test | yes (tests only) | test report + updated tests |
| `review` | review | no | findings + severity |
| `documentation` | after merge | yes (docs) | changelog, docs PR |

Every agent has a **tool allowlist** and a **budget**; the runtime rejects a call outside the allowlist
(`403 tool_not_permitted`) and stops the run on budget (`budget_exceeded`).

## 7. Artifacts and where they live

| Artifact | Storage | Bound to | Hash |
|---|---|---|---|
| `plan` (markdown) | `run_artifacts` (S3) | run | yes |
| `diff` (unified) | `run_artifacts` | run | yes |
| `test_report` (JSON) | `run_artifacts` | run + commit | yes |
| `log` (NDJSON) | `run_events` + S3 archive | run | yes |
| `context_manifest` | `run_artifacts` | run | yes |
| `review_findings` | table `review_findings` | run | no |
| `release` | `releases` | project | tag |
| `audit` | `audit_log` (append-only) | org | — |

## 8. Metrics (and how they are measured)

| Metric | Definition | Source |
|---|---|---|
| Lead time | `task.ready_at` → `task.done_at` | `tasks` |
| Cycle time per phase | duration of `run_phases` | `run_phases` |
| First-pass yield | % of runs that passed G2 on the first attempt | `runs` |
| Escaped defects | bugs found after `done` and linked to the task | `tasks.parent_id` |
| Cost per task | Σ `usage_ledger.cost_micros` / task | `usage_ledger` |
| Review latency | from the G3 request to the decision | `approvals` |
| Human touch ratio | number of human actions / number of tasks | `audit_log` |

Every metric in the UI carries `source`: `measured` (from the database), `estimated` (from a model,
e.g. a forecast), `n/a` (no data). Never an "invented" number.

## 9. Failure handling

| Situation | Behavior |
|---|---|
| Provider 429/5xx | exponential backoff (max 5), then `failed` with `retryable=true` |
| Tool error | 2 retries, then the phase is `failed`; the run stays at the same checkpoint |
| Test fail after 3 iterations | run `failed`, the diff is kept as an artifact for the human |
| Approval timeout | `expired`; the task returns to `ready` with a comment |
| Worker crash | River re-dispatch; the phase continues from the last checkpoint |
| Budget exceeded | `failed` + `budget_exceeded`; partial diff saved |

## 10. How this repo goes through the same process

| ASDLC step | Here specifically |
|---|---|
| Blueprint | `docs/02-architecture.md` + `docs/06-design-system.md` |
| Backlog | `docs/05-mvp-plan.md` (epics/stories) |
| DoR | checklist in [AGENTS.md](../AGENTS.md#1-definition-of-ready-for-a-story) |
| Plan gate | the plan is written into the PR description before the code |
| Test gate | `make check` = fmt + vet + unit + integration + tsc + openapi diff |
| Review gate | PR template with 5 questions; a `blocker` blocks the merge |
| Learn | after every milestone: ADR + update of this document |

## 11. Process anti-patterns

- ❌ "The agent decided on its own to expand the scope" → ACs change only through a task update.
- ❌ "The tests failed but the diff looks good" → G2 is not advice, it is a gate.
- ❌ "I will approve the plan in advance for all tasks" → approval is per run, not per epic.
- ❌ "We will add audit later" → audit events are written from day one; they are not reconstructed retroactively.
- ❌ "The metric is an estimate, we will show it as a measurement" → `source` is required.
