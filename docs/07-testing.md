# 07 · Testing

## 1. Pyramid

```
        ┌──────────────┐  e2e (Playwright)      ~15 scenarios, critical flows only
        ├──────────────┤  contract (OpenAPI)    every endpoint, no network
        ├──────────────┤  integration (Go)      repo + DB + queue, real Postgres
        ├──────────────┤  replay (agent)        recorded LLM responses, determinism
        └──────────────┘  unit (Go/TS)          domain, validations, components
```

Target coverage: `internal/domain` ≥ 90%, `internal/app` ≥ 80%, `internal/httpapi` ≥ 70%.
**We do not chase the number** — a story without a test for a new invariant is not done regardless of the %.

## 2. Go — what goes where

| Level | Location | Tool | What is tested |
|---|---|---|---|
| unit | `*_test.go` next to the code, `package foo` | `testing`, `cmp.Diff` | domain invariants, state machine, validations |
| integration | `internal/platform/db/*_integration_test.go` | real Postgres via `AIP_TEST_DB_*` (CI service container) | SQL, RLS, indexes, transactions |
| queue | `internal/adapters/queue` | River against the same test database | retry, unique jobs, crash recovery |
| contract | `internal/httpapi/contract_test.go` | `httptest` + `oapi-codegen` validation | every endpoint returns the schema from `openapi.yaml` |
| replay | `internal/agent/replay_test.go` | golden files | same input → same plan/diff |

### 2.1. Table-driven, no magic

```go
func TestTaskTransition(t *testing.T) {
    cases := []struct{ name string; from, to domain.TaskStatus; wantErr error }{
        {"ready→planning", domain.TaskReady, domain.TaskPlanning, nil},
        {"review→planning", domain.TaskReview, domain.TaskPlanning, domain.ErrInvalidTransition},
        {"draft→done", domain.TaskDraft, domain.TaskDone, domain.ErrInvalidTransition},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { /* ... */ })
    }
}
```

### 2.2. Integration: a real database, never a mock

```go
//go:build integration

func TestTenantIsolation(t *testing.T) {
    appDSN := os.Getenv("AIP_TEST_DB_DSN")
    migDSN := os.Getenv("AIP_TEST_DB_MIGRATOR_DSN")
    if appDSN == "" || migDSN == "" {
        t.Skip("AIP_TEST_DB_DSN / AIP_TEST_DB_MIGRATOR_DSN not set")
    }
    ctx := context.Background()
    require.NoError(t, db.Migrate(ctx, migDSN, migrations.FS, "up"))

    app, err := db.Connect(ctx, appDSN)      // role aip_app, no BYPASSRLS
    require.NoError(t, err)
    t.Cleanup(app.Close)

    var n int
    db.WithOrg(ctx, app, orgA, func(ctx context.Context, tx pgx.Tx) error {
        return tx.QueryRow(ctx, "SELECT count(*) FROM memberships").Scan(&n)
    })
    require.Equal(t, 1, n)                   // only org A is visible
}
```
CI provides Postgres as a service container, creates the `aip_app` / `aip_migrator`
roles from `deploy/postgres/00_roles.sql` and runs `aipctl migrate up` before the tests.
The RLS test is mandatory: querying without `WithOrg` must return 0 rows, and
`UPDATE audit_log` as `aip_app` must fail with `permission denied`.

### 2.3. Mock only at the edge

LLM, sandbox and GitHub are mocked **at the port** (`Provider`, `Sandbox`, `GitHubClient`),
never in the domain. The mock provider returns recorded chunks from `testdata/llm/*.ndjson`.

## 3. Replay harness (the most important test for the agent)

```
internal/agent/testdata/replay/{scenario}/
  input.json          # task, rules snapshot, repo sha, model
  llm.ndjson          # recorded provider responses (per call)
  tools.ndjson        # recorded tool results
  expected/plan.md
  expected/diff.patch
  expected/events.json # types + order, no timestamps
```

Run: `AIP_REPLAY=1 go test ./internal/agent -run TestReplay` — no network, no sandbox.
If the prompt template changes, `expected/*` is reviewed and updated **by hand** (golden
diff), never automatically.

## 4. Contract tests

- `openapi.yaml` is the source; `oapi-codegen` generates the server interface.
- `openapi-diff` in CI: breaking change → fail (the `breaking-change` label unblocks it with an ADR).
- Every handler tests: success, `400`, `401`, `403`, `404`, `409` where applicable.
- SSE test: `httptest` + reading the stream; verification of `id:`, `event:` and backfill.

## 5. E2E (Playwright)

Only 15 scenarios, each must be stable 50× in a row:

1. login → dashboard
2. create project from a blueprint (wizard)
3. create task + DoR validation
4. start planning run + live log
5. approve plan → implement
6. diff review + approve
7. test failure → message + retry
8. cancel run
9. board drag & drop changes status
10. task detail tests tab
11. rules edit + publish + context tab shows the version
12. environments deploy + prod approval
13. audit filter
14. dark/light toggle + a11y (axe-core, 0 critical)
15. 375px: task list as cards

Playwright uses `data-testid` (stable), not text (translations change).

## 6. Flaky policy

- A test fails 3× in CI → `quarantine` label, an issue **immediately**, owner for 2 days.
- `retry` only for `known_flaky` (the list in `e2e/quarantine.txt`), never globally.
- The quarantine list must not exceed 5 tests; beyond that, feature work stops.

## 7. Security checklist (before every production deploy)

- [ ] Sandbox: no docker socket, non-root, read-only rootfs, `cap-drop ALL`, NetworkPolicy
- [ ] Egress: allowlist; test "agent attempts `curl https://example.com`" → blocked
- [ ] Secrets: no plaintext in the database; the API never returns `secret_ref`
- [ ] Prompt injection: the tool allowlist is honored even when the model asks for something else
- [ ] RLS: the test from §2.2 passes
- [ ] Rate limit: `POST /runs` returns `429` after the limit
- [ ] Webhook: HMAC verification, constant-time compare, timestamp tolerance
- [ ] Audit: every state change has a record
- [ ] Dependency scan (govulncheck, trivy) with no `HIGH`
- [ ] Logs contain no prompt content and no tokens at INFO level

## 8. CI pipeline

```yaml
# .github/workflows/ci.yml (sketch)
jobs:
  go:
    steps:
      - go vet ./...
      - golangci-lint run
      - go test -short ./...            # unit, fast
      - go test -tags=integration ./...  # real Postgres service
      - go test -race ./internal/agent/...
      - govulncheck ./...
  web:
    steps: [pnpm lint, pnpm tsc --noEmit, pnpm test, pnpm build]
  contract:
    steps: [oapi-codegen --check, openapi-diff origin/main]
  e2e:
    needs: [go, web]
    steps: [docker compose up -d, pnpm playwright test --repeat-each=1]
```

`make check` locally = the same sequence without e2e (e2e is `make e2e`).

## 9. Test data

- Seed: `aipctl seed --org acme --demo` → 1 org, 3 projects, 12 tasks, 2 runs
  (one `awaiting_approval`, one `failed`) — so the UI has all states without manual clicking.
- No test depends on production data or the network (except the provider in `replay`
  mode, which does not go out to the network).

## 8. Load test (E10-S4)

`scripts/k6/loadtest.js` drives the public API with real HTTP: each iteration
creates a task, moves it to `ready` and starts a development run. Thresholds are
p95 < 300 ms and < 5% failed requests.

```bash
# k6 from https://github.com/grafana/k6/releases (or `make loadtest-k6 K6=...`)
AIP_BASE=http://localhost:18080 RUNS=50 VUS=10 k6 run scripts/k6/loadtest.js
```

The per-org active-run cap (`AIP_MAX_ACTIVE_RUNS`, default 5) is respected: a
429 is counted as `runs_throttled`, not as a failure. To start all 50 at once,
raise the cap above `RUNS` for the target API (the local run above used a
dedicated API container with the command agent backend to avoid LLM cost).

Last measured run (2026-10-24, 50 runs / 154 requests): p95 **118.67 ms**, 0
failed requests, 50/50 runs started.
