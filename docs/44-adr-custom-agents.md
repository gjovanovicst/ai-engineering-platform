# ADR 0002 — Custom agent plugins

Status: **implemented and proven end-to-end with the fake-GitLab e2e.**

## Context

The platform already runs external agents through the command/sandbox backend
(Claude Code, Codex, in-house CLIs) and MCP tools. Phase E calls for a plugin SDK
with custom agents. "Custom agent" here means a **registered agent
implementation** that a project can select, not a model prompt.

## Options

| Option | Pros | Cons |
|---|---|---|
| A. Command image + prompt | already works; no new contract | no structured metadata, no per-plugin policy |
| B. HTTP agent service | language-agnostic, easy scaling | network egress, auth, workspace transfer |
| C. Go plugin (`.so`) | in-process | brittle, unsafe, Go-version coupled |

## Decision (proposed)

Take **A as the execution model** and add a registry/contract around it:
a plugin is an OCI image + metadata, executed in the existing sandbox with the
same limits. Do not load `.so` plugins in-process.

## Contract (v1)

Metadata (registered by an org admin):

```json
{
  "name": "acme-reviewer",
  "version": "1.2.0",
  "image": "ghcr.io/acme/reviewer@sha256:...",
  "agent_types": ["review"],
  "command": ["/agent", "--prompt", "/workspace/.aip/prompt.txt"],
  "allowed_tools": ["read_file", "list_files"],
  "max_wall_clock_s": 900,
  "max_cost_micros": 500000
}
```

Runtime guarantees:

- fresh sandbox, no host mounts, no network unless the project policy allows it
- the run's frozen context and budget are passed as today
- the plugin cannot call the platform API; it only reads the workspace/prompt and
  writes the workspace, and emits the same `AIP_*` lines
- the platform still owns checks, approvals, merge and audit

## Registration and selection

- `agent_plugins` per org (name, version, image digest, metadata, status)
- `projects.agent_plugin_id` optional; empty keeps the built-in backend
- only admins register; only owners/admins select per project
- selecting a plugin with a mutable tag is rejected; digest required

## Out of scope (v1)

- plugins calling back into the platform
- plugin-provided UI
- in-process Go plugins
- marketplace/signing beyond image cosign verification

## Test plan before "done"

1. A plugin image runs a planning/development run with the same gates as built-ins.
2. A plugin cannot read the host, other runs, secrets, or the platform API.
3. A project with a plugin still enforces required checks/approvals.
4. A digest change requires re-approval; a tag is rejected.
5. Kill/timeout leaves no running container and records a failure code.
6. Plugin output cannot bypass `AIP_PROOF`/acceptance verification.

## Consequences

- New schema and admin UI; a larger E item, not a `plugin_checks` slice.
- Supply-chain: plugin images need cosign verification like the API image.


## Implementation status (2026-11-05)

- migration 0072: `agent_plugins` (org-scoped, digest required) and
  `projects.agent_plugin_id`
- admin API: `POST/GET /orgs/{org}/agent-plugins`, `PUT
  /orgs/{org}/projects/{project}/agent-plugin`
- Settings UI and a worker project-backend selector that builds a
  `CommandBackend` from the plugin image/command and only for the plugin's
  agent types
- live: register (201), select (204), project JSON exposes `agent_plugin_id`,
  clear (204)
- **proven:** `scripts/e2e.sh` registers a digest-pinned plugin, selects it for
  the project, and the full plan → development → PR → merge flow succeeds through
  the plugin backend (`[e2e] OK ...`)
- Settings → Agent plugins: register, list, select/clear per project; no cosign
  verification of plugin images yet


## Signed-image drill (2026-11-05)

- local `registry:2`; sandbox image pushed and signed with a generated cosign key
  pair (`cosign sign --tlog-upload=false`)
- cosign is now in the API image; the API mounted the public key and used the
  verifier hook against the registry
- live: signed plugin registration **201**; unsigned image **400**
  (`image signature verification failed`)
- the drill registry/key mount were removed afterwards; production must drop
  `--allow-insecure-registry` and use a real registry/identity
- production use must drop `--allow-insecure-registry`; the key-based path keeps
  it for the local drill
