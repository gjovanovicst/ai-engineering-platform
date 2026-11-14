# 26 · Design Standards & Component Registry (proposal)

Status: **implemented** — E12-S1…S6, component allowlist, forbidden/import compliance, designer role, structured standard builder, component preview and per-project exceptions. Platform follow-ups: CI check sync and acceptance verifier · 2026-10-25 · new cycle after M0–M5 and P2

## 1. Why

Today `Design System` and `Components` are organisation-level registries:

- `design_tokens` (name/value/category) and `design_components`
  (name/purpose/rules/status) with CRUD at `/orgs/{org}/design/*`;
- `projects.design_system_id` exists in the schema but is never read;
- `docs/09-blueprint-schema.md` already describes `design_system`
  `{ref, version, theme, components[]}`, but the validator ignores it;
- the agent prompt carries the task, the approved plan and the published rules —
  **no design tokens, no approved components, no UI constraints**.

So the pages describe *our* UI today, not the standards that the projects and the
agents must follow. The redesign handoff's task composer already accepts “design
references”, which is a natural entry point.

The goal is to make design standards executable context for agents, not a static
catalogue:

```
Organisation standards ──► Project standards ──► Agent prompt ──► diff ──► compliance check ──► merge gate
```

## 2. Target model

| Level | What it defines | Where |
|---|---|---|
| **Organisation** | design standard versions (tokens, typography, spacing, radius, themes, a11y/responsive rules) and the component registry (approved/deprecated/forbidden, package, import path, examples) | `design_standards`, `design_components` |
| **Project** | which standard version applies, component allowlist/overrides, stack mapping (React/Vue/Angular), exceptions with a reason | `projects.design_standard_id/version`, `project_components` |
| **Agent / run** | standards injected into the prompt and the context manifest; a `design.snapshot` event; a compliance check on the diff | prompt builder, `context.manifest`, `test_results` |

Key rules:

- only **published** standard versions can be assigned to a project;
- published versions are immutable (same trigger pattern as blueprints/rules);
- every agent run records *which* version and component set it used (audit);
- compliance violations are visible and can block the merge through the existing
  required-checks gate.

## 3. Stories

### E12-S1 · Design standard versions (M) — implemented 2026-10-25

- migration: `design_standards(id, org_id, name, version, status, spec jsonb,
  sha256, published_at, created_at)` + `unique(org_id, name, version)` +
  immutability trigger on published rows.
- `spec` (v1): `tokens[{name,value,category}]`, `typography`, `spacing`,
  `radius`, `themes{light,dark}`, `accessibility[]`, `responsive[]`,
  `patterns[]` (form/table/navigation rules).
- API: `GET/POST /orgs/{org}/design-standards`,
  `POST .../{id}/versions`, `POST .../{id}/publish`.
- UI: Design Standards page with version list, editor (structured form +
  JSON preview), publish and diff between versions.
- **AC**: assigning a draft is rejected; publishing freezes the row; two
  versions can be diffed; audit entries exist.

### E12-S2 · Component registry 2.0 (M) — implemented 2026-10-25

- extend `design_components`: `category`, `framework`, `package`, `import_path`,
  `version`, `docs`, `example`, `status` (`approved|deprecated|forbidden`),
  `design_standard_id` (optional) and version rows or an append-only history.
- API: list with filters (`category`, `status`, `framework`), create/update,
  publish/deprecate, delete only drafts.
- UI: registry with filters, code snippet + copy, states and a deprecation
  banner; “Add to project” action.
- **AC**: a forbidden/deprecated component cannot be added to a project
  allowlist; every entry shows package/import path and an example.

### E12-S3 · Project standards (M) — implemented 2026-10-25 (standard assignment + component allowlist)

- migration: `project_components(project_id, component_id, mode
  allow|exception, reason, created_by)`; use existing
  `projects.design_system_id` + new `design_standard_version`.
- API: `GET/PUT /orgs/{org}/projects/{project}/standards`.
- UI: Project → **Standards** tab: inherited design standard + version, component
  allowlist, stack mapping, exceptions with reason, compliance status.
- Blueprint instantiate copies `design_system.ref/version` into the project.
- **AC**: only published versions; exceptions require a reason; project page
  shows exactly what the agent will receive.

### E12-S4 · Agent context injection (M) — implemented 2026-10-25

- `BuildPrompt` (or a context assembler) appends a compact standards block:
  token names/values, layout/pattern rules, allowed components with import paths,
  forbidden libraries/components, accessibility rules.
- `context.manifest` gains `design_standard {id, version, sha256}` and the
  component ids; a `design.snapshot` event mirrors it.
- Artifact `context_manifest` stores the block; prompt stays deterministic.
- **AC**: a run records the standard version; changing the project standard
  changes the manifest hash; the prompt contains the allowed components and the
  forbidden list; absent standards do not change behaviour.

### E12-S5 · Design compliance check (M) — implemented 2026-10-25 (raw colours + forbidden imports)

- after the development diff, a deterministic scanner checks:
  raw hex colours where tokens are required, imports of unapproved UI packages,
  usage of forbidden/deprecated components, missing `alt`/`aria-label` on new
  interactive elements (best effort), and custom spacing values when a scale is
  mandated.
- each violation `{rule, path, line, message, severity}` is stored as a
  `test_results` row named `design`; the existing required-check gate can block
  the merge.
- API to list violations; run/PR UI shows them; waivers require a reason and are
  audited.
- **AC**: a diff with a raw hex fails the check; a compliant diff passes; the
  merge gate blocks when `design` is required.

### E12-S6 · IA / navigation (S) — implemented 2026-10-25

- rename the group to **Developer Tools** (or **Standards**) and move Design
  Standards + Components there; visible read-only to members, writable by
  admin/owner; hide the group from users without access if a role split lands.
- Project Standards page linked from the project header and from the task
  composer's “design references”.
- **AC**: everyday users still see Projects/Agents/Analytics; developers find
  standards in one place; no empty “catalogue” pages.

## 4. API / schema sketch

```sql
design_standards(id, org_id, name, version, status, spec jsonb, sha256,
                 published_at, created_at, updated_at,
                 unique(org_id, name, version))

design_components(id, org_id, design_standard_id, name, category, framework,
                  package, import_path, version, purpose, rules, docs, example,
                  status, created_at, updated_at)

projects.design_system_id  -> design_standards.id (nullable)
projects.design_standard_version int
project_components(project_id, component_id, mode, reason, created_by, created_at)
```

All tables get RLS with `app_org_id()`; writes are audited; published rows are
immutable.

## 5. Compatibility with existing docs

- implement the `design_system` block already described in
  `docs/09-blueprint-schema.md` instead of inventing a second format;
- keep the current token/component registries as the data source, migrated into
  the versioned standard (data migration: create a v1 standard per org from the
  existing rows);
- the rules system stays for engineering conventions; design standards are a
  separate, typed context block.

## 6. Out of scope (for this cycle)

- Figma import/sync;
- visual regression screenshots in CI;
- automatic code generation of components;
- per-component semver resolution across frameworks (we store the version and
  import path, not a resolver).

## 7. Risks

| Risk | Mitigation |
|---|---|
| Prompt bloat increases cost | cap the block (e.g. 2k tokens), summarise tokens by category, allow a project to select a subset |
| False positives in compliance | start with high-precision rules (hex colours, imports, forbidden components), waivers with audit |
| Migration of existing registries | create v1 draft per org, publish after review; keep old tables read-only until migrated |
| Standards drift | published versions immutable; project records version; run records hash |

## 8. Suggested order

1. E12-S1 + E12-S2 (model + registry UI)
2. E12-S3 (project assignment)
3. E12-S4 (agent context)
4. E12-S5 (compliance + gate)
5. E12-S6 (IA polish)

Each story follows the normal DoR, `make check`, a screenshot for UI and a
rollback path. Recommended first slice: **E12-S1 + E12-S3 read path** so a
project can display “this standard applies” before agents consume it.
