# 09 · Blueprint schema

The blueprint is the **only entry point** into creating a project. This document defines the `spec` JSON
stored in `blueprint_versions.spec` — without it, the wizard, `POST /projects` and the agent context
have nothing to validate against.

- Schema version: `"1"` (changed only together with an ADR).
- Validation: JSON Schema 2020-12 (server: `santhosh-tekuri/jsonschema/v6`).
- A blueprint version is **immutable** once published; a change = a new version.

## 1. JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://aip.dev/schemas/blueprint-v1.json",
  "type": "object",
  "required": ["schema_version", "name", "stack", "repository", "test_strategy", "agents"],
  "additionalProperties": false,
  "properties": {
    "schema_version": { "const": "1" },
    "name": { "type": "string", "minLength": 3, "maxLength": 80 },
    "description": { "type": "string", "maxLength": 2000 },

    "stack": {
      "type": "object",
      "required": ["frontend", "backend"],
      "additionalProperties": false,
      "properties": {
        "frontend": {
          "type": "object",
          "required": ["framework", "language"],
          "properties": {
            "framework": { "enum": ["next", "vite-react", "angular", "vue", "sveltekit", "none"] },
            "version": { "type": "string" },
            "language": { "enum": ["typescript", "javascript"] },
            "styling": { "enum": ["tailwind", "css-modules", "styled-components", "none"] },
            "ui_kit": { "type": "string" },
            "package_manager": { "enum": ["pnpm", "npm", "yarn"], "default": "pnpm" }
          }
        },
        "backend": {
          "type": "object",
          "required": ["language"],
          "properties": {
            "language": { "enum": ["go", "node", "python", "java", "none"] },
            "version": { "type": "string", "examples": ["1.25"] },
            "framework": { "type": "string", "examples": ["chi", "echo", "gin", "stdlib"] },
            "database": { "enum": ["postgres", "mysql", "sqlite", "none"] },
            "queue": { "enum": ["river", "asynq", "none"] },
            "storage": { "enum": ["s3", "local", "none"] }
          }
        },
        "infra": {
          "type": "object",
          "properties": {
            "container": { "enum": ["docker", "none"], "default": "docker" },
            "ci": { "enum": ["github-actions", "gitlab-ci", "none"], "default": "github-actions" },
            "orchestrator": { "enum": ["k8s", "compose", "none"], "default": "compose" }
          }
        }
      }
    },

    "repository": {
      "type": "object",
      "required": ["layout", "default_branch"],
      "properties": {
        "layout": { "enum": ["monorepo", "polyrepo"] },
        "default_branch": { "type": "string", "default": "main" },
        "commit_convention": { "enum": ["conventional", "free"], "default": "conventional" },
        "paths": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "examples": [{ "web": "web", "api": "cmd", "internal": "internal", "docs": "docs" }]
        }
      }
    },

    "design_system": {
      "type": "object",
      "properties": {
        "ref": { "type": "string", "description": "design system ID from the registry" },
        "version": { "type": "string" },
        "theme": { "enum": ["light", "dark", "system"], "default": "system" },
        "components": {
          "type": "array",
          "items": { "type": "string", "pattern": "^[a-z0-9-]+@[0-9]+\\.[0-9]+\\.[0-9]+$" },
          "examples": [["button@2.4.0", "input@1.8.0", "data-table@3.1.0"]]
        }
      }
    },

    "module_map": {
      "type": "array",
      "description": "Maps modules to paths; used by context assembly and DoR.",
      "items": {
        "type": "object",
        "required": ["module", "paths"],
        "properties": {
          "module": { "type": "string", "examples": ["auth", "billing"] },
          "paths": { "type": "array", "items": { "type": "string" } },
          "owners": { "type": "array", "items": { "type": "string" } }
        }
      }
    },

    "test_strategy": {
      "type": "object",
      "required": ["gates"],
      "properties": {
        "frameworks": {
          "type": "object",
          "properties": {
            "unit": { "type": "string", "examples": ["go test", "vitest"] },
            "e2e": { "type": "string", "examples": ["playwright"] }
          }
        },
        "coverage": {
          "type": "object",
          "properties": {
            "min_new_code": { "type": "number", "minimum": 0, "maximum": 100, "default": 80 },
            "min_delta": { "type": "number", "default": -1 }
          }
        },
        "gates": {
          "type": "object",
          "required": ["required_suites"],
          "properties": {
            "required_suites": { "type": "array", "items": { "type": "string" } },
            "fail_on_coverage_drop": { "type": "boolean", "default": true },
            "max_duration_s": { "type": "integer", "default": 1800 },
            "e2e_smoke": { "type": "array", "items": { "type": "string" } }
          }
        }
      }
    },

    "agents": {
      "type": "object",
      "required": ["defaults"],
      "properties": {
        "defaults": {
          "type": "object",
          "required": ["model"],
          "properties": {
            "model": { "type": "string", "examples": ["gpt-4o", "claude-sonnet-4-5"] },
            "budget": {
              "type": "object",
              "properties": {
                "tokens_in": { "type": "integer", "default": 400000 },
                "tokens_out": { "type": "integer", "default": 120000 },
                "cost_micros": { "type": "integer", "default": 5000000 },
                "wall_clock_s": { "type": "integer", "default": 2400 }
              }
            }
          }
        },
        "tools": {
          "type": "object",
          "description": "Allowlist per agent type (see 11-agent-runtime.md).",
          "additionalProperties": {
            "type": "array",
            "items": { "type": "string" }
          },
          "examples": [{
            "planning": ["fs.list", "fs.read", "search.code", "git.status", "plan.submit"],
            "development": ["fs.read", "fs.write", "search.code", "git.diff", "git.commit", "shell.exec", "test.run"]
          }]
        },
        "prompts": {
          "type": "object",
          "additionalProperties": { "type": "string" },
          "examples": [{ "plan": "plan/v3", "implement": "implement/v5" }]
        }
      }
    },

    "rules": {
      "type": "array",
      "description": "Initial rules copied into the org on instantiation (version 1).",
      "items": {
        "type": "object",
        "required": ["level", "name", "body"],
        "properties": {
          "level": { "enum": ["organization", "project", "module", "task"] },
          "name": { "type": "string" },
          "body": { "type": "string" }
        }
      }
    },

    "environments": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": { "enum": ["dev", "staging", "prod"] },
          "auto_deploy": { "type": "boolean", "default": false },
          "requires_approval": { "enum": ["none", "deploy_prod"], "default": "none" }
        }
      },
      "default": [{ "name": "dev", "auto_deploy": true }, { "name": "staging", "auto_deploy": true }]
    },

    "task_defaults": {
      "type": "object",
      "properties": {
        "priority": { "enum": ["low", "med", "high"], "default": "med" },
        "require_dor": { "type": "boolean", "default": true }
      }
    }
  }
}
```

### 1.1. Validation rules outside the schema

1. If `backend.language = "go"`, `backend.framework` must be in `["chi","echo","gin","stdlib"]`.
2. If `frontend.framework = "none"`, `design_system` must be absent.
3. `design_system.components[].version` must exist in the components registry (`components` table).
4. At least one env must be `prod` **or** an explicit `"environments": []` (CLI-only product).
5. `test_strategy.gates.required_suites` must not be empty if the project has a backend.
6. The sum of the default budgets must not exceed the org limit (`orgs.max_concurrent_runs` is unrelated; the limit is `billing.plan.runs_budget`).

## 2. Example A — SaaS (Next.js + Go)

```json
{
  "schema_version": "1",
  "name": "saas-next-go",
  "description": "Multi-tenant SaaS: Next.js frontend, Go API, Postgres, River queue.",
  "stack": {
    "frontend": { "framework": "next", "version": "15", "language": "typescript",
                  "styling": "tailwind", "ui_kit": "shadcn", "package_manager": "pnpm" },
    "backend": { "language": "go", "version": "1.25", "framework": "chi",
                 "database": "postgres", "queue": "river", "storage": "s3" },
    "infra": { "container": "docker", "ci": "github-actions", "orchestrator": "compose" }
  },
  "repository": {
    "layout": "monorepo", "default_branch": "main", "commit_convention": "conventional",
    "paths": { "web": "web", "api": "cmd", "internal": "internal", "migrations": "migrations", "docs": "docs" }
  },
  "design_system": { "ref": "acme-ds", "version": "2.4.0", "theme": "system",
                     "components": ["button@2.4.0", "input@1.8.0", "data-table@3.1.0", "modal@1.2.0"] },
  "module_map": [
    { "module": "auth", "paths": ["internal/app/auth", "web/src/app/(auth)"] },
    { "module": "billing", "paths": ["internal/app/billing", "web/src/app/(app)/billing"] }
  ],
  "test_strategy": {
    "frameworks": { "unit": "go test", "e2e": "playwright" },
    "coverage": { "min_new_code": 80, "min_delta": -1 },
    "gates": { "required_suites": ["go-test", "web-unit", "web-e2e-smoke"],
               "fail_on_coverage_drop": true, "max_duration_s": 1800,
               "e2e_smoke": ["login", "create-project", "run-agent"] }
  },
  "agents": {
    "defaults": { "model": "gpt-4o",
                  "budget": { "tokens_in": 400000, "tokens_out": 120000,
                              "cost_micros": 5000000, "wall_clock_s": 2400 } },
    "tools": {
      "planning": ["fs.list", "fs.read", "search.code", "git.status", "plan.submit"],
      "development": ["fs.read", "fs.write", "search.code", "git.diff", "git.commit", "shell.exec", "test.run"],
      "testing": ["fs.read", "fs.write", "test.run"],
      "review": ["fs.read", "git.diff", "search.code"]
    },
    "prompts": { "plan": "plan/v3", "implement": "implement/v5", "review": "review/v2" }
  },
  "rules": [
    { "level": "organization", "name": "security",
      "body": "No eval. No secrets in code. Sandbox has no production network access." },
    { "level": "organization", "name": "testing",
      "body": "Minimum 80% coverage for new modules. Every PR runs e2e smoke." }
  ],
  "environments": [
    { "name": "dev", "auto_deploy": true },
    { "name": "staging", "auto_deploy": true },
    { "name": "prod", "auto_deploy": false, "requires_approval": "deploy_prod" }
  ],
  "task_defaults": { "priority": "med", "require_dor": true }
}
```

## 3. Example B — internal tool (React + Node)

```json
{
  "schema_version": "1",
  "name": "internal-tool-react-node",
  "stack": {
    "frontend": { "framework": "vite-react", "language": "typescript", "styling": "tailwind" },
    "backend": { "language": "node", "framework": "fastify", "database": "postgres", "queue": "none" },
    "infra": { "container": "docker", "ci": "github-actions", "orchestrator": "compose" }
  },
  "repository": { "layout": "monorepo", "default_branch": "main",
                  "paths": { "web": "apps/web", "api": "apps/api" } },
  "test_strategy": {
    "frameworks": { "unit": "vitest", "e2e": "playwright" },
    "coverage": { "min_new_code": 70, "min_delta": -2 },
    "gates": { "required_suites": ["vitest", "e2e-smoke"], "fail_on_coverage_drop": true, "max_duration_s": 1200 }
  },
  "agents": {
    "defaults": { "model": "claude-sonnet-4-5" },
    "tools": { "development": ["fs.read", "fs.write", "search.code", "git.commit", "shell.exec", "test.run"] }
  },
  "environments": [{ "name": "dev", "auto_deploy": true }]
}
```

## 4. Instantiation: `spec` → concrete records

`POST /orgs/{org}/projects` with `{ "blueprint_id": ..., "version": "1.0.0" }` in a single
transaction creates the following:

| From `spec` | Results in |
|---|---|
| `name` + the user-provided name | a `projects` row, `blueprint_id` + `blueprint_version` |
| `stack` | `projects.tech_stack[]` (e.g. `["next","go","postgres"]`) |
| `repository` | a `repos` row (connected through the GitHub App) + skeleton commit (template) |
| `rules[]` | `rules` + `rule_versions` (version 1, `published_at` = now) |
| `module_map` | `projects.module_map` (jsonb) — used by context assembly |
| `test_strategy.gates` | `projects.test_gates` (jsonb) — read by the G2 gate |
| `agents.tools` | `projects.tool_policy` (jsonb) — allowlist per agent type |
| `agents.defaults.budget` | default `runs.budget` |
| `design_system.ref` | `projects.design_system_ref` (FK to `design_systems`) |
| `environments[]` | `environments` rows (`dev`, `staging`, `prod`) |
| `task_defaults` | default values when creating a task |

Everything copied into the project is a **snapshot**: a later change to the blueprint does not change
the existing project (except an explicit "re-apply blueprint", which creates a new version and a diff).

## 5. Versioning

| State | Rule |
|---|---|
| draft | changed freely, not visible in the wizard |
| published | immutable; `published_at` + `sha256`; the wizard offers only published |
| deprecated | new fields forbidden; can still be instantiated for 90 days |
| schema_version | a bump = a migration (`aipctl blueprint migrate`) |

`sha256` is computed over the canonicalized JSON (sorted keys, no whitespace) and
written to `blueprint_versions.spec_sha256`. The run records `blueprint_version` + the hash, so
it is known exactly which blueprint the project originated from.

## 8. Schema v2 — starter tasks

Schema version `"2"` adds one optional property. A version `"1"` spec stays
valid and is treated as having no starter tasks.

```json
{
  "schema_version": "2",
  "starter_tasks": [
    {
      "title": "Set up CI",
      "description": "Run make check on every PR.",
      "priority": "p1",
      "acceptance": [
        { "given": "a pull request", "when": "it is opened", "then": "CI runs make check" }
      ]
    }
  ]
}
```

Rules:

- `title` is required, 3–120 characters; `description` ≤ 5000.
- `priority` is `p0|p1|p2`, default `p2`.
- `acceptance` entries have `given`, `when`, `then` (all non-empty).
- Tasks are created in `draft`; a human moves them to `ready` after the DoR.
- Instantiation is project + tasks; if a task fails, the project stays and the
  response reports how many tasks were created.
