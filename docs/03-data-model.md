# 03 · Data model

PostgreSQL 17. All tables are `org_id`-scoped except the global ones (`users`,
`model_prices`, `ai_providers`). Primary keys are UUID (`gen_random_uuid()`); times are
`timestamptz` in UTC. No soft delete except where explicitly noted (`projects`, `tasks`);
the audit log keeps history.

## 1. ER overview

```
orgs ──< memberships >── users
  │
  ├──< projects ──< tasks ──< runs ──< run_phases
  │      │            │         ├──< run_events
  │      │            │         ├──< tool_calls
  │      │            │         ├──< approvals
  │      │            │         └──< run_artifacts
  │      │            └──< task_links (deps)
  │      ├──< repos ──< repo_credentials (1:1)
  │      │        └──< pull_requests ──< test_results
  │      ├──< environments ──< deployments
  │      └──< rules ──< rule_versions
  ├──< blueprints ──< blueprint_versions
  ├──< ai_providers / model_prices (global)
  ├──< usage_ledger
  └──< audit_log

users ──< sessions / api_keys
```

## 2. Enum types

```sql
CREATE TYPE org_role        AS ENUM ('owner','admin','member','viewer');
CREATE TYPE task_status     AS ENUM ('draft','ready','planning','in_progress','review','done','blocked','canceled');
CREATE TYPE task_priority   AS ENUM ('low','med','high');
CREATE TYPE run_status      AS ENUM ('queued','running','awaiting_approval','succeeded','failed','canceled');
CREATE TYPE run_phase_name  AS ENUM ('context','plan','implement','test','review','diff','pr');
CREATE TYPE phase_state     AS ENUM ('pending','active','done','failed','skipped');
CREATE TYPE agent_type      AS ENUM ('planning','development','testing','review','documentation');
CREATE TYPE approval_type   AS ENUM ('plan','pr','deploy_prod','rule_publish');
CREATE TYPE approval_status AS ENUM ('pending','approved','rejected','changes_requested','expired');
CREATE TYPE data_source     AS ENUM ('measured','estimated','na');
CREATE TYPE env_name        AS ENUM ('dev','staging','prod');
CREATE TYPE deploy_status   AS ENUM ('queued','running','succeeded','failed','rolled_back');
CREATE TYPE severity        AS ENUM ('blocker','major','minor','nit');
```

## 3. Schema

```sql
-- ── Tenancy & identity ─────────────────────────────────────────────────
CREATE TABLE orgs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
  name        text NOT NULL,
  max_concurrent_runs int NOT NULL DEFAULT 4,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email         citext NOT NULL UNIQUE,
  name          text NOT NULL,
  password_hash text,                              -- NULL for OIDC-only accounts
  avatar_url    text,
  email_verified_at timestamptz,
  last_login_at timestamptz,
  must_change_password boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       org_role NOT NULL DEFAULT 'member',
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, user_id)
);

CREATE TABLE invites (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  email      citext NOT NULL,
  role       org_role NOT NULL DEFAULT 'member',
  token_hash bytea NOT NULL,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  created_by uuid NOT NULL REFERENCES users(id)
);

CREATE TABLE sessions (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_hash  bytea NOT NULL UNIQUE,             -- sha256(refresh token)
  org_id        uuid REFERENCES orgs(id) ON DELETE SET NULL,  -- active org for this session
  role          text CHECK (role IN ('owner','admin','member','viewer')),
  platform      text NOT NULL DEFAULT 'web' CHECK (platform IN ('web','mobile','cli')),
  user_agent    text,
  ip            inet,
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  revoked_reason text,
  last_used_at  timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON sessions (user_id) WHERE revoked_at IS NULL;

-- email verification and password reset (see 12-auth.md)
CREATE TABLE email_verifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE password_resets (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  requested_ip inet,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_attempts (
  id         bigserial PRIMARY KEY,
  email_hash bytea NOT NULL,          -- sha256(lower(email)); plaintext is never stored
  ip         inet,
  success    boolean NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON login_attempts (email_hash, created_at DESC);

CREATE TABLE user_identities (
  provider   text NOT NULL,           -- github | google
  subject    text NOT NULL,           -- OIDC sub
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, subject)
);

CREATE TABLE api_keys (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name       text NOT NULL,
  key_hash   bytea NOT NULL UNIQUE,
  scopes     text[] NOT NULL DEFAULT '{}',
  last_used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ── Blueprint / projects / tasks ───────────────────────────────────────
CREATE TABLE blueprints (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, name)
);

CREATE TABLE blueprint_versions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  blueprint_id uuid NOT NULL REFERENCES blueprints(id) ON DELETE CASCADE,
  version      text NOT NULL,                      -- semver: 1.0.0
  spec         jsonb NOT NULL,                     -- stack, design_system, tests, rules, tools
  published_at timestamptz,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (blueprint_id, version)
);

CREATE TABLE projects (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name           text NOT NULL,
  slug           text NOT NULL,
  description    text NOT NULL DEFAULT '',
  blueprint_id   uuid REFERENCES blueprints(id),
  blueprint_version text,
  tech_stack     text[] NOT NULL DEFAULT '{}',
  status         text NOT NULL DEFAULT 'active',
  archived_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, slug)
);
CREATE INDEX ON projects (org_id, status) WHERE archived_at IS NULL;

CREATE TABLE tasks (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id      uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  ref             text NOT NULL,                   -- "T-88", per project
  title           text NOT NULL,
  description     text NOT NULL DEFAULT '',
  acceptance      jsonb NOT NULL DEFAULT '[]',     -- [{given,when,then}]
  status          task_status NOT NULL DEFAULT 'draft',
  priority        task_priority NOT NULL DEFAULT 'med',
  tags            text[] NOT NULL DEFAULT '{}',
  assignee_id     uuid REFERENCES users(id),
  parent_id       uuid REFERENCES tasks(id),
  blocked_reason  text,
  created_by      uuid NOT NULL REFERENCES users(id),
  ready_at        timestamptz,
  done_at         timestamptz,
  deleted_at      timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, ref)
);
CREATE INDEX ON tasks (project_id, status, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON tasks (assignee_id) WHERE deleted_at IS NULL;

CREATE TABLE task_links (
  from_task uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  to_task   uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  kind      text NOT NULL CHECK (kind IN ('blocks','relates','duplicate')),
  PRIMARY KEY (from_task, to_task, kind),
  CHECK (from_task <> to_task)
);

CREATE TABLE comments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  author_id  uuid REFERENCES users(id),
  body       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ── Runs (event-sourced part) ──────────────────────────────────────────
CREATE TABLE runs (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id     uuid NOT NULL REFERENCES projects(id),
  task_id        uuid REFERENCES tasks(id),
  agent_type     agent_type NOT NULL,
  status         run_status NOT NULL DEFAULT 'queued',
  current_phase  run_phase_name,
  model          text NOT NULL,
  provider_id    uuid REFERENCES ai_providers(id),
  budget         jsonb NOT NULL,                   -- {tokens_in,tokens_out,cost_micros,wall_clock_s}
  spent          jsonb NOT NULL DEFAULT '{"tokens_in":0,"tokens_out":0,"cost_micros":0}',
  rules_snapshot jsonb NOT NULL,                   -- [{rule_id, version, sha256}]
  base_commit    text,
  head_commit    text,
  cancel_requested_at timestamptz,
  started_at     timestamptz,
  finished_at    timestamptz,
  failure_code   text,
  failure_detail text,
  created_by     uuid REFERENCES users(id),
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX one_active_run_per_task ON runs (task_id)
  WHERE status IN ('queued','running','awaiting_approval') AND task_id IS NOT NULL;
CREATE INDEX ON runs (org_id, status, created_at DESC);
CREATE INDEX ON runs (project_id, created_at DESC);

CREATE TABLE run_phases (
  run_id     uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  phase      run_phase_name NOT NULL,
  attempt    int NOT NULL DEFAULT 1,
  state      phase_state NOT NULL DEFAULT 'pending',
  started_at timestamptz,
  ended_at   timestamptz,
  meta       jsonb NOT NULL DEFAULT '{}',
  PRIMARY KEY (run_id, phase, attempt)
);

-- append-only; seq is per-run and is assigned under a row lock on runs
CREATE TABLE run_events (
  run_id      uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  seq         bigint NOT NULL,
  type        text NOT NULL,      -- run.started, phase.started, llm.chunk, tool.call, ...
  actor       text NOT NULL CHECK (actor IN ('agent','human','system')),
  payload     jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, seq)
);
CREATE INDEX ON run_events (run_id, created_at);

CREATE TABLE tool_calls (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id        uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  seq           int NOT NULL,
  phase         run_phase_name NOT NULL,
  tool          text NOT NULL,
  args          jsonb NOT NULL,
  result        jsonb,
  permitted     boolean NOT NULL,
  duration_ms   int,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);

CREATE TABLE run_artifacts (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id      uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('plan','diff','log','test_report','context_manifest','screenshot')),
  bucket_key  text NOT NULL,
  sha256      bytea NOT NULL,
  bytes       bigint NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE approvals (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  run_id       uuid REFERENCES runs(id) ON DELETE CASCADE,
  deployment_id uuid,
  type         approval_type NOT NULL,
  status       approval_status NOT NULL DEFAULT 'pending',
  requested_at timestamptz NOT NULL DEFAULT now(),
  decided_at   timestamptz,
  decided_by   uuid REFERENCES users(id),
  reason       text,
  expires_at   timestamptz
);
CREATE INDEX ON approvals (org_id, status, requested_at DESC);

-- ── Rules (versioned) ──────────────────────────────────────────────────
CREATE TABLE rules (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
  level      text NOT NULL CHECK (level IN ('organization','project','module','task')),
  scope_ref  text,                                  -- module/task ref for narrower levels
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rule_versions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_id      uuid NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
  version      int NOT NULL,
  body         text NOT NULL,
  sha256       bytea NOT NULL,
  published_at timestamptz,
  published_by uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (rule_id, version)
);

-- ── Git / quality ──────────────────────────────────────────────────────
CREATE TABLE repos (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  provider     text NOT NULL DEFAULT 'github',
  full_name    text NOT NULL,                       -- acme/crm-platform
  default_branch text NOT NULL DEFAULT 'main',
  installation_id bigint,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, full_name)
);

-- One encrypted credential per repository. Resolution order is
-- repo_credentials -> GitHub App installation token -> global AIP_GITHUB_TOKEN.
CREATE TABLE repo_credentials (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  repo_id    uuid NOT NULL UNIQUE REFERENCES repos(id) ON DELETE CASCADE,
  ciphertext bytea NOT NULL,                        -- AES-GCM envelope from AIP_SECRETS_KEY
  hint       text NOT NULL DEFAULT '',              -- last four characters, for the UI
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE pull_requests (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  repo_id       uuid NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  run_id        uuid REFERENCES runs(id),
  task_id       uuid REFERENCES tasks(id),
  number        int NOT NULL,
  url           text NOT NULL,
  head_sha      text NOT NULL,
  state         text NOT NULL,                      -- open/closed/merged
  merged_at     timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repo_id, number)
);

CREATE TABLE test_results (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  run_id      uuid REFERENCES runs(id) ON DELETE CASCADE,
  suite       text NOT NULL,
  total       int NOT NULL,
  passed      int NOT NULL,
  failed      int NOT NULL,
  skipped     int NOT NULL DEFAULT 0,
  duration_ms int NOT NULL,
  coverage_pct numeric(5,2),
  report_key  text,                                 -- S3 key of the full report
  commit_sha  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE review_findings (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id     uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  file       text NOT NULL,
  line       int,
  severity   severity NOT NULL,
  body       text NOT NULL,
  resolved   boolean NOT NULL DEFAULT false
);

-- ── Environments & deploy ──────────────────────────────────────────────
CREATE TABLE environments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name       env_name NOT NULL,
  url        text,
  config     jsonb NOT NULL DEFAULT '{}',
  UNIQUE (project_id, name)
);

CREATE TABLE deployments (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
  release_ref    text NOT NULL,                     -- tag or sha
  status         deploy_status NOT NULL DEFAULT 'queued',
  deployed_by    uuid REFERENCES users(id),
  started_at     timestamptz,
  finished_at    timestamptz,
  log_key        text
);

-- ── Providers, usage, audit ────────────────────────────────────────────
CREATE TABLE ai_providers (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL UNIQUE,                  -- openai-compatible / anthropic
  base_url   text NOT NULL,
  secret_ref text NOT NULL,                         -- reference to the secret store, not plaintext
  enabled    boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE model_prices (
  provider_id  uuid NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
  model        text NOT NULL,
  context_window int NOT NULL,
  input_micros_per_1m  bigint NOT NULL,
  output_micros_per_1m bigint NOT NULL,
  valid_from   timestamptz NOT NULL,
  PRIMARY KEY (provider_id, model, valid_from)
);

CREATE TABLE usage_ledger (
  id          bigserial PRIMARY KEY,
  org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  project_id  uuid REFERENCES projects(id),
  run_id      uuid REFERENCES runs(id) ON DELETE SET NULL,
  provider_id uuid REFERENCES ai_providers(id),
  model       text NOT NULL,
  tokens_in   int NOT NULL,
  tokens_out  int NOT NULL,
  cost_micros bigint NOT NULL,                      -- USD * 1e6
  source      data_source NOT NULL DEFAULT 'measured',
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON usage_ledger (org_id, created_at);
CREATE INDEX ON usage_ledger (run_id);

CREATE TABLE audit_log (
  id         bigserial PRIMARY KEY,
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  actor_id   uuid REFERENCES users(id),
  actor_kind text NOT NULL CHECK (actor_kind IN ('user','api_key','system')),
  action     text NOT NULL,                         -- task.transition, run.approve, ...
  target     text NOT NULL,                         -- task:T-88, run:uuid
  metadata   jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON audit_log (org_id, created_at DESC);

CREATE TABLE flags (
  org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name   text NOT NULL,
  value  boolean NOT NULL,
  PRIMARY KEY (org_id, name)
);
```

-- ── Design registries (screens 6, 7, 8) ───────────────────────────────
CREATE TABLE design_systems (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, name)
);

CREATE TABLE design_system_versions (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  design_system_id uuid NOT NULL REFERENCES design_systems(id) ON DELETE CASCADE,
  version          text NOT NULL,                 -- semver
  tokens           jsonb NOT NULL,                -- identical to the structure in 06-design-system.md §1.1
  sha256           bytea NOT NULL,
  published_at     timestamptz,
  published_by     uuid REFERENCES users(id),
  UNIQUE (design_system_id, version)
);

CREATE TABLE components (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id           uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  design_system_id uuid REFERENCES design_systems(id) ON DELETE SET NULL,
  name             text NOT NULL,                 -- "Button"
  slug             text NOT NULL,                 -- "button"
  category         text NOT NULL,                 -- layout | nav | data | status | form | feedback | overlay
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, slug)
);

CREATE TABLE component_versions (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  component_id  uuid NOT NULL REFERENCES components(id) ON DELETE CASCADE,
  version       text NOT NULL,
  status        text NOT NULL DEFAULT 'beta' CHECK (status IN ('stable','beta','deprecated')),
  props         jsonb NOT NULL DEFAULT '[]',      -- [{name,type,default,description}]
  events        jsonb NOT NULL DEFAULT '[]',
  a11y_notes    text,
  ai_notes      text,                             -- guidance for the AI using the component
  preview_url   text,
  sha256        bytea NOT NULL,
  published_at  timestamptz,
  UNIQUE (component_id, version)
);

-- ── Notifications, webhooks, deploy logs (screens 1, 11, 13) ──────────
CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       text NOT NULL,          -- approval.requested, run.failed, deployment.succeeded
  title      text NOT NULL,
  body       text NOT NULL DEFAULT '',
  target     text,                   -- run:uuid | task:T-88
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON notifications (user_id, read_at, created_at DESC);

CREATE TABLE webhook_endpoints (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  url         text NOT NULL,
  secret_hash bytea NOT NULL,
  events      text[] NOT NULL DEFAULT '{}',
  enabled     boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deployment_logs (
  deployment_id uuid NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
  seq           bigint NOT NULL,
  line          text NOT NULL,
  level         text NOT NULL DEFAULT 'info',
  created_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (deployment_id, seq)
);

-- ── Project inherits the snapshot from the blueprint (see 09-blueprint-schema.md §4) ──
ALTER TABLE projects
  ADD COLUMN module_map        jsonb NOT NULL DEFAULT '[]',
  ADD COLUMN test_gates        jsonb NOT NULL DEFAULT '{}',
  ADD COLUMN tool_policy       jsonb NOT NULL DEFAULT '{}',
  ADD COLUMN design_system_id  uuid REFERENCES design_systems(id),
  ADD COLUMN blueprint_sha256  bytea;

ALTER TABLE blueprint_versions
  ADD COLUMN spec_sha256 bytea;

## 4. Rules enforced by the database (not only the application)

1. **One active run per task** — partial unique index `one_active_run_per_task`.
2. **`run_events` is append-only** — `REVOKE UPDATE, DELETE` for the application role;
   a correction is a new event of type `*.corrected`.
3. **`audit_log` is append-only** — same; retention 24 months, archive in S3 (WORM bucket).
4. **Tenancy** — RLS on all tenant tables:

```sql
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY tasks_tenant ON tasks
  USING (org_id = app_org_id());   -- app_org_id() = nullif(current_setting('app.org_id', true), '')::uuid

-- A user must see their own memberships before an org context exists (login):
CREATE POLICY memberships_self ON memberships
  USING (user_id = app_user_id());  -- app_user_id() = nullif(current_setting('app.user_id', true), '')::uuid
```
   At the start of every transaction the application runs `SET LOCAL app.org_id = $1`. The
   `aip_app` role **does not have** `BYPASSRLS`; migrations run under `aip_migrator`.

## 5. Indexes and performance — rules

- No list endpoint without `LIMIT`; pagination is keyset (`(created_at, id) < ($1,$2)`).
- Dashboard aggregates: materialized view `mv_org_daily` (runs, cost, test pass),
  `REFRESH MATERIALIZED VIEW CONCURRENTLY` every 5 min from a worker.
- `jsonb` fields (`acceptance`, `budget`, `spec`, `rules_snapshot`) are not searchable except
  where a GIN index exists; for the MVP there is no GIN except on `blueprint_versions.spec`
  (search by technology in the wizard).

## 6. Retention

| Table | Period | After |
|---|---|---|
| `run_events` | 90 days | NDJSON archive in S3 (cold), deleted from the database |
| `run_artifacts` | 12 months | S3 lifecycle → Glacier |
| `audit_log` | 24 months | S3 WORM |
| `usage_ledger` | permanent | aggregated, never deleted |
| `sessions` | 30 days after expiry | deleted |
