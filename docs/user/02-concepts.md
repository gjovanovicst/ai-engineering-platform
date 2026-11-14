# Key concepts

The platform implements an **AI Software Development Lifecycle (ASDLC)**: a sequence of phases, artifacts and gates. The model is replaceable; the process is the product.

## Lifecycle at a glance

```
idea → blueprint → backlog → ready → plan → approve → build → test → review → PR approval → merge → measure
```

Each arrow is a state transition. The platform records who or what caused it.

## Phases

| Phase | Owner | Output |
|---|---|---|
| Blueprint | human + agent | project template: stack, repository layout, tests, agents |
| Backlog | human | tasks with context and acceptance criteria |
| Definition of ready | human | a task that can safely start |
| Plan | planning agent | steps, files, risks, estimated cost |
| Build | development agent | a diff in an isolated sandbox |
| Test | agent + platform | test report, design checks, acceptance evidence |
| Review | human + agent | findings, PR comments, approval or rejection |
| Merge | human | a merged change in the Git provider |
| Measure | platform | delivery and quality metrics |

## Gates

A gate is a deliberate stop. It cannot be skipped silently; an explicit override is recorded as an event.

1. **Plan approval** — a human accepts the plan before implementation.
2. **Test gate** — required checks must pass; acceptance evidence must be valid.
3. **Design gate** — design standards and visual checks for UI changes.
4. **PR approval** — a human approves the pull request.
5. **Merge queue** — changes land in a serialised order.

## Artifacts

Every artifact is versioned and hashed:

- **context manifest** — the exact files, rules and token budget given to the agent
- **plan** — steps, files, risks and cost estimate
- **diff** — the change produced by the run
- **test report** — unit, integration, e2e and visual results
- **review** — findings and their severity
- **audit event** — append-only record of a decision or transition

## Roles

| Role | Typical responsibilities |
|---|---|
| Owner | workspace billing, members, security settings |
| Admin | projects, providers, rules, integrations, audit |
| Member | tasks, runs, reviews, pull requests |
| Viewer | read-only access to projects and results |

Roles are enforced by the API and the UI; project-level access can be narrower than the workspace role.

## Runs

A run is one agent execution with a type: **planning**, **development** or **review**. Runs are resumable at phase boundaries and stop at approvals. A run has a budget, a frozen context and an append-only event stream.

## Two kinds of speed

The platform reports delivery speed **and** outcome quality:

- lead time and approval time
- first-pass merge and proof validity
- rework, change failure rate and escaped defects

A faster merge that creates more rework is not an improvement.

## Next

- [Projects and blueprints](projects-blueprints.html)
- [Runs and approvals](runs-and-approvals.html)
- [Quality gates](quality.html)
