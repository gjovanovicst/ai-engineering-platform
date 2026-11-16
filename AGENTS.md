# AGENTS.md — rules for the AI building this repo

You are an implementer, not an architect. The architecture is decided in `docs/`; your job is to
carry it out consistently and to **not invent** what is not defined.

## 0. Reading order (required, before the first `write`)

1. `docs/01-asdlc.md` — process and gates (what "done" means).
2. `docs/02-architecture.md` — where code lives and why.
3. `docs/03-data-model.md` — what you may persist.
4. `docs/04-api.md` + `api/openapi.yaml` — the contract.
5. `docs/06-design-system.md` + `docs/10-components.md` — if you touch the UI.
6. `docs/12-auth.md` — if you touch auth/sessions; `docs/13-setup.md` — environments and secrets.
7. The story from `docs/05-mvp-plan.md` — the exact scope.

If the story is not in the plan: **do not implement it**. Propose it as a new story.

## 1. Definition of Ready for a story

Do not start unless you can answer all 6:

- [ ] Which packages/files change (list them before the code)?
- [ ] Which endpoint/schema changes, and is it a breaking change?
- [ ] Which state (loading/empty/error) is new, if this is UI?
- [ ] Which test proves it is done?
- [ ] What is **out** of scope?
- [ ] How is it rolled back (migration? feature flag?)

If any of this is unclear, ask. A question costs a minute; a wrong assumption costs a day.

## 2. Go rules

Go paths in this document are relative to `apps/api/` (the module root); web paths are
relative to `apps/web/`.

- **No I/O in `internal/domain`.** No `pgx`, `http`, `os` in the domain. The domain is pure.
- **Errors are wrapped:** `fmt.Errorf("task transition: %w", err)`; the domain has sentinel
  errors (`ErrNotFound`, `ErrInvalidTransition`), the HTTP mapping lives in one place.
- **`context.Context` is the first parameter** and goes through every layer. No `context.TODO()`.
- **No `panic`** in application code. `panic` only in `main` for invalid config
  (fail fast at startup).
- **Transactions:** one use case = one transaction; `SELECT ... FOR UPDATE` wherever a
  state change could be touched by another goroutine.
- **SQL only through sqlc** (except migrations). No string concatenation in queries.
- **Never `SELECT *`** in production code; explicit columns (sqlc requires that anyway).
- **Logging:** `slog` with structured fields; never `fmt.Println`, never secrets.
- **Goroutines:** every one has an owner and a way to stop (`ctx` or `close(done)`).
  No "fire and forget" without `errgroup` or an explicit lifecycle.
- **Interfaces:** define them on the consumer side (`app/ports.go`), not in the adapter.
  An interface with one method is fine; an interface with 10 methods is a sign of a bad boundary.
- **Tests:** table-driven, `t.Run`, `t.Parallel()` where there is no shared state.

## 3. TypeScript / React rules

- `strict: true`; no `any` (use `unknown` + narrowing).
- Server state: TanStack Query; **no** `useEffect` fetches.
- Forms: `react-hook-form` + zod schema from OpenAPI types where possible.
- Components: one component = one file; `components/ui/*` are primitives, with no
  business logic.
- **No hex color** in `components/`; tokens only (see design system §9).
- All lists: `loading | empty | error | ready` (design system §4).
- A11y: `aria-label` on icon buttons, focus ring, Escape for overlays.

## 4. Forbidden (hard stop)

- ❌ Changing `api/openapi.yaml` without regenerating code and without an `openapi-diff` pass.
- ❌ A new migration that alters an existing one (forward-only; rollback = a new migration).
- ❌ `UPDATE`/`DELETE` on `run_events` or `audit_log`.
- ❌ Sending prompt content or secrets to logs/telemetry.
- ❌ Merge or deploy without human approval (even if the tests pass).
- ❌ "Fixing" a test to pass by weakening the assertion.
- ❌ Adding a dependency without an ADR (unless it is already in `go.mod`).
- ❌ Committing directly to `main`.

## 5. Commit and PR

```
<type>(<scope>): <imperative, ≤ 72 characters>

Why: <1–2 sentences of context>
What: <what changed>
Risk: <low|medium|high> — <what can break>
Rollback: <how it is reverted>
Run-Id: <uuid>            # if it originated from an agent run
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `infra`.
Scope = package or module (`tasks`, `runs`, `agent/sandbox`, `web/backlog`).

Every `feat`, `fix` and `perf` PR updates `CHANGELOG.md` under Unreleased. CI
enforces this; for a justified exception put `[skip-changelog]` in the PR
title.

The PR description contains 5 required sections: **What / Why / How tested / Risk and rollback /
Screenshot** (for UI). Without them the PR is not ready for review.

## 6. Verification before "done"

```bash
make check      # go fmt + vet + lint + unit + integration + tsc + lint web + openapi diff
make e2e        # if you touched a flow covered by e2e
```

If `make check` does not pass, it is not done. If it passes and you doubt the coverage,
write a test, not an explanation.

## 7. How to ask (without blocking)

Ask **immediately** if:
- the spec does not define behavior the story requires,
- two stories conflict,
- the change would be breaking for the API or schema.

Do not ask about:
- things that are in `docs/` (read again),
- variable naming,
- formatting (`gofmt`/`prettier` handles it).

Phrase the question as: **context → options (A/B) → recommendation → consequences**.

## 8. What counts as "excellent" in this repo

- Code that reads as if one person wrote it, not five agents.
- A test that would fail if someone broke the invariant; not a test that covers a line.
- A 300-line PR with a clear description, not 3000 lines of "refactoring".
- No `TODO` without an issue link.

## 9. Backlog: how an agent picks up a story

Until M4 the backlog is **GitHub Issues** (the platform does not exist yet). The labels are defined in
[docs/13-setup.md §8](docs/13-setup.md#8-backlog-while-the-platform-does-not-exist).

### 9.1. Pickup protocol

1. Filter: `is:issue is:open label:ready -label:blocked` + `label:agent:go` or `agent:web`.
2. Take the **oldest** by `p0` > `p1` > `p2`, then by the order in `05-mvp-plan.md`.
   Do not pick the "more interesting" one; the order is part of the plan.
3. Assign yourself + comment `picked up: <plan of 3–5 steps>`.
4. If the DoR is not met (no AC, no scope, no test) → label `blocked`,
   a comment with exactly one question, and **move on to the next one**.
5. WIP limit: **1 story per agent**. Parallel work is possible only on different
   labels (`agent:go` vs `agent:web`) and with no shared files.

### 9.2. Branch and PR

```
feat/{issue-number}-{short-slug}       # feat/123-task-drawer
```

- PR required: `Closes #123`, the 5 sections from §5, a screenshot for UI.
- If the PR touches `openapi.yaml`, state `breaking: yes|no` in the description and run `make check`.
- Request review from a human; the agent does **not** approve its own PR and does **not** merge.

### 9.3. Statuses

| When | What |
|---|---|
| start | assign + `picked up` comment |
| blocked | label `blocked` + reason + `@` decision owner |
| PR open | comment with a link; the issue stays open |
| merge | `Closes` closes the issue automatically |
| abandoning | unassign + comment `not doing: <reason>` (do not delete the story) |

### 9.4. What not to do

- ❌ Do not open a PR larger than ~400 lines of changes (except migrations/generated code).
- ❌ Do not take two stories "because they are small"; the WIP limit is 1.
- ❌ Do not change `05-mvp-plan.md` to get a story through; propose a new story through an issue.
- ❌ Do not commit `.env`, keys, database dumps, `node_modules`.
