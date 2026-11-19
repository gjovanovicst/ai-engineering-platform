# Roadmap — v0.2.0 → v1.0.0

**Status:** v0.1.1 released (2026-11-18). The current release: **v0.2.0**.

This is the plan of record. The live state lives in the GitHub milestones and
issues; this file is the map: themes, order, and the full issue list per
release.

## Principles

- **6–8 stories per feature release** (plus a small process story or two).
- **Patch releases between features**: `0.2.1`, `0.2.2`, `0.3.1`… only fixes,
  additive migrations, no new features, no breaking changes.
- **The pilots are the test bench**: each release is exercised on real pilot
  apps before and after the release; their findings feed the patches.
- **Every story meets the Definition of Ready** before it is picked up
  (`AGENTS.md` §1) and ships with the test that proves it.

## Release plan

| Release | Theme | Status |
|---|---|---|
| v0.1.0 | Developer preview | ✅ released |
| v0.1.1 | Self-hosting readiness | ✅ released |
| v0.2.0 | UX + Assistant | **in progress** |
| v0.3.0 | Design 2.0 + Blueprint 2.0 | planned |
| v0.4.0 | The Stack (multi-language/framework) | planned |
| v0.5.0 | The Flow (agents, skills, workflow) | planned |
| v0.6.0 | Agent Quality (eval, context, review) | planned |
| v0.7.0 | Production / Enterprise | planned |
| v1.0.0 | GA | planned |

Dependency order: design/blueprint → stack → flow → quality → production.

## v0.2.0 — UX + Assistant

### Features

1. **Task Composer 2.0** — a full window (context, basics, acceptance builder,
   scope, dependencies, attachments, DoR, actions), preview, templates, AI
   draft; the same component for edit.
2. **Task scope fields + blueprint task templates** — migration for
   files/out-of-scope/test/rollback/estimate/assignee; blueprint seed backlog;
   instantiation creates the tasks.
3. **Forms foundation** — react-hook-form + zod, field primitives, inline
   validation, the key forms migrated, a11y.
4. **Guidance layer** — contextual "what's next", tooltips/glossary, help
   panel, empty states, error remediation.
5. **Command palette actions + shortcuts** — actions (create task, start run,
   navigate, open settings), recent items, keyboard.
6. **AI assistant MVP** — side panel; explain a failure/review; draft a task;
   docs Q&A; create task/retry with confirmation; RBAC and audit.
7. **Run/PR UX polish** — sticky actions, timeline, cost/usage, review/checks
   hierarchy, approval UX, mobile.
8. **Onboarding setup wizard** — repository, test command, sandbox image and
   provider, in one guided flow.

### Process

9. **Patch-release mechanism** — bug template v2, severity/backport labels,
   `release/vX.Y` branches, backport action, nightly workflow.
10. **Pilots** — `pilot-go` and `pilot-web` repos, seed tasks, weekly cadence,
    pilot report template.

## v0.3.0 — Design 2.0 + Blueprint 2.0

1. **Component playground + contract 2.0** — variants, props, states, a11y,
   examples; live preview; code snippet; contract diff.
2. **Token system 2.0** — primitive → semantic → component, themes, WCAG
   contrast, export, alias/deprecation.
3. **Design review gate + component enforcement** — designcheck in the review
   gate; approved/forbidden components; token/a11y rules.
4. **Visual regression approve/reject** — baseline diff UI, approve as
   baseline, history, masks.
5. **Figma sync** — diff/pull/push for tokens and components, review of the
   changes.
6. **Blueprint 2.0** — design standard, rules, task templates and policies in
   the blueprint; project defaults; lineage.
7. **Blueprint gallery, preview and versioning** — gallery, instantiate
   preview, version diff, upgrade.
8. **Design system docs site** — generated from the registry (components,
   tokens, standards), usage, changelog.

## v0.4.0 — The Stack

1. **Stack catalog** — model, API, UI, seeded stacks, custom org stacks.
2. **Project/blueprint stack selection + runtime** — commands, image, context
   and defaults follow the stack.
3. **Per-stack sandbox images** — Python, Java, .NET, Rust, Ruby, PHP build
   and publish pipeline; docs; custom images.
4. **Dependency preparation per ecosystem** — pip, Maven/Gradle, Cargo,
   Bundler, Composer, NuGet; offline cache; `dependency.fetch` events.
5. **Multi-language repo map and symbols** — Python, Java, C#, Rust, Ruby, PHP.
6. **Test report parsers** — JUnit XML, pytest, jest/vitest, `go test`, Maven
   Surefire, NUnit, RSpec; coverage.
7. **Design per framework** — React, Vue, Angular, Svelte contracts, token
   export, review rules.
8. **Project templates per stack** — starters, seed tasks, docs.

## v0.5.0 — The Flow

1. **Agent profiles + versioning** — name, role, model, prompt, tools, skills,
   sandbox, limits, triggers; draft → test → publish; rollback.
2. **Agent Studio UI** — list, editor, test run, publish, per-project
   assignment, diff.
3. **Command plugin polish** — BYO image/command, cosign digest allowlist,
   sandbox test, docs.
4. **Skills library** — built-in skills (migrations, review, docs, API, UI,
   a11y) and custom org/project skills, versioned.
5. **Skills editor + run snapshot** — editor/preview, agent skill picker, run
   snapshot of skills and versions, audit.
6. **Workflow builder** — stages, agents/skills, gates, conditions, preview.
7. **Runtime respects the workflow** — stages, skips, custom agents, gates.
8. **Blueprint integration** — workflow, agents and skills in the blueprint;
   instantiation; project defaults.

## v0.6.0 — Agent Quality

1. **Eval suite + metrics** — fixtures, runner (CLI/CI), success/steps/cost/
   review precision.
2. **Replay harness** — `aipctl replay`, recorded runs, regressions.
3. **Context + repo map at scale** — symbol index, changed-file priority,
   adaptive budget, stuck detection.
4. **Review calibration** — severity guidance, chunked diffs, dedupe,
   precision metrics.
5. **Test-failure feedback** — the gate failure returns to the agent within the
   same run.
6. **Fix loop** — repeat fix, max attempts, notifications, human handoff.
7. **Cost + usage optimization** — model routing, prompt caching, budgets,
   analytics.

## v0.7.0 — Production / Enterprise

1. **Audit witness UI + verification**.
2. **Backup + restore drill** — CLI/UI, RPO/RTO, tests.
3. **Quotas + cost controls** — org budgets, per-project limits, alerts.
4. **Observability dashboards** — run metrics, Prometheus, alerts.
5. **SSO/SAML/SCIM polish** — enterprise settings, sync, docs.
6. **HA + fencing drill** — Patroni fencing, failover, runbook.
7. **Multi-region + BYOK pilot**.
8. **Egress policy + sandbox hardening**.

## v1.0.0 — GA

1. **Release process + versioning policy** — semver, cadence, changelog
   automation, upgrade guide.
2. **User guide + API reference final**.
3. **Support diagnostics bundle** — `aipctl diag`, log bundle, health.
4. **Final UX, a11y and performance** — audit, Lighthouse, polish.
5. **SLO, stability and upgrade path**.
6. **Security review + pen-test fixes**.
7. **Launch checklist** — site, demo, docs, announcement.
8. **v1.0.0 release** — tag, images, GitHub Release, milestone.

## Patch releases (0.x.y)

Between feature releases, patches ship **fixes only**:

- **Branches**: `main` carries the next feature; `release/vX.Y` is frozen at the
  feature tag and receives backports (`cherry-pick -x`).
- **Flow**: bug issue → triage (severity, repro, target) → `fix:` PR on main
  with a regression test → CI + review → merge → backport to `release/vX.Y` →
  `CHANGELOG` + `docs/releases/vX.Y.Z.md` → tag → images and GitHub Release.
- **Severity**: S1 hotfix within 24h from the release branch; S2 next weekly
  patch; S3 next feature release; S4 backlog.
- **Migrations**: additive only; the upgrade path is tested by the nightly.

## Pilots

The pilots are the bug-discovery bench between sprints:

| Pilot | Stack | Covers |
|---|---|---|
| ChangeFlow | Next/Node, in-memory | API + UI, e2e, review/CI fix |
| pilot-go | Go API + Postgres | Go tests, migrations, per-project test command |
| pilot-web | Next + design system | design/components, visual, a11y |
| pilot-py | FastAPI/Django | another ecosystem, pytest, migrations |

- Weekly: one real task on each pilot through the platform.
- Release end: a bug bash on the pilots (new features and edge cases).
- The pilot report (run ids, outcome, friction, bugs, time, cost) feeds the
  patch backlog and, from v0.6.0, the eval fixtures.

## How we track

- **GitHub milestones** per release (`v0.2.0`, `v0.3.0`, …, plus a milestone per
  patch when it opens).
- **GitHub issues** per story, labelled `story`, `ready`, `pN`, `agent:*`;
  bugs labelled `bug`, `severity:s1..s4`, `regression`, `backport:vX.Y`.
- **This file** is the plan of record; the milestones are the live state.
- **Release notes**: `docs/releases/<tag>.md`; the changelog entry per release.

## Status legend

- ✅ released
- 🚧 in progress
- ⬜ planned
