# The pilots

The pilot workspaces are the test bench. Each one is a real application with
real tasks; the platform runs them for a week and the findings feed the patch
releases (see `ROADMAP.md`, "Patch releases between features").

Current pilots:

| Pilot | Stack | Status |
|---|---|---|
| `changeflow` | Next.js + Postgres | running |
| `pilot-go` | Go API + Postgres | to register |
| `pilot-web` | Next.js + the design system | to register |

## The setup checklist (each pilot)

A pilot is ready when all four are true in its workspace:

1. **A project** — the pilot's own project (the pilot tasks live there, never in
   `Demo`).
2. **A repository** — the GitHub repository, its default branch and the webhook
   secret (`POST /orgs/{org}/projects/{project}/repos`).
3. **A test command** — the command the development run must pass
   (`PATCH /orgs/{org}/projects/{project}` with `test_command`).
4. **A sandbox image** — the operator sets `AIP_SANDBOX_IMAGE`; the pilot
   confirms it runs the stack's tools.

The `/setup` wizard walks exactly these four steps.

## The weekly cadence

| Day | What |
|---|---|
| Monday | Pick one task from the pilot's `docs/tasks.md`; move it to ready. |
| Tuesday | The agent plans; a human approves or rejects. |
| Wednesday | The development run; a human reviews the diff. |
| Thursday | The gates and the merge (a human merges). |
| Friday | The report: the outcome, the friction, the bugs. |

## The report template

Each week the pilot keeps one report (`docs/pilots/reports/<pilot>-<week>.md`):

```md
# <pilot> — week <ISO week>

## The task
- ref: <T-n>
- outcome: merged | rejected | retried | blocked

## The run
- run id: <uuid>
- the gates: <passed|failed>
- the human decision: <approved|rejected|n/a>

## The friction
- what slowed the run down (the plan, the context, the tools)

## The bugs
- one line per defect, with the severity (`severity:s1`..`s4`) and the
  reproduction steps
```

## Registering a new pilot

1. Create the repository (it needs the platform's GitHub App or a token with the
   `repo` scope).
2. Create the project in the platform.
3. Open `/setup` and complete the four steps.
4. Add `docs/tasks.md` to the pilot with three ready tasks.

## Rollback

Archive the pilot repository and the project; the platform keeps the audit.
