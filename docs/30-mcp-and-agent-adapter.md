# MCP tools and external agent adapters

Status: **in progress** · 2026-10-26 · follows docs/28 §A6
Implemented: phase 1 (stdio client + fake-server test) the sandbox `StartProcess` (Docker; Kubernetes not-supported) and the MCP manager/namespacing (`mcp__server__tool`, built-ins cannot be shadowed) and tool-loop dispatch in both the OpenAI and Anthropic loops. Server configuration (`AIP_MCP_SERVERS`), the sandbox start helper and per-run lifecycle wiring in both backends are implemented; failed servers emit `mcp.unavailable`. Project-level `projects.mcp_servers` overrides the environment config per run; Delivery → Checks shows the configured server chips, validates the JSON in preflight, and can load the tools discovered by the latest development run (`GET /projects/{id}/mcp-tools`, event `mcp.tools`). A sandbox-backed fake-server test covers start → list → call. Each MCP call emits `mcp.call` with server/tool/status/duration; preflight validates the project MCP JSON. Per-server egress remains blocked on a separate sandbox/network per server.

## Goal

Let a project use Model Context Protocol servers and external coding agents
(Claude Code, DeepSeek) without weakening the proof/approval gates.

Non-goals: a general plugin marketplace, network MCP servers in P0, or letting an
MCP server bypass the sandbox.

## MCP

### Transport

Start with **stdio**, newline-delimited JSON-RPC 2.0 (the MCP stdio framing),
because it is local, testable and needs no inbound network.

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"aip","version":"dev"}}}
```

Sequence per run: `initialize` → `tools/list` → zero or more `tools/call` →
shutdown/kill. Timeouts: initialize 5s, list 5s, call 60s; a hung server is
killed and the run continues without its tools (audited event).

### Configuration

Project-level (later), environment-level first:

```bash
AIP_MCP_SERVERS='[{"name":"fs","command":"mcp-fs","args":["--root","/workspace"]}]'
```

Only **trusted operator configuration** may start a process; models cannot add
servers. Secrets are passed via env allowlist, never the prompt.

### Security

- MCP servers run **inside the run sandbox**, not on the host. Each server now gets its own sandbox so it can have its own network policy (`allow_network`), implemented via `StartMCPIsolated`.
- Network disabled by default; an explicit per-server allowlist is required.
- Filesystem access is the workspace mount only.
- Every discovered tool is namespaced `mcp__<server>__<tool>` to prevent shadowing
  built-ins (`read_file`, `write_file`, `run_tests`).
- Tool output is truncated (20k) and scanned for secrets before being stored.
- A server that fails to initialize is recorded as `mcp.unavailable`; the run is
  not silently degraded without an event.

### Integration

`toolSpecs` and `execute` are shared by the OpenAI and Anthropic loops. Both get
a `toolProvider`:
- `specs(ctx)` returns built-ins + MCP tools;
- `call(ctx, name, args)` dispatches built-ins, then MCP namespaces.
The tool allowlist test is extended to assert MCP tools cannot shadow built-ins.

### Acceptance

1. A fake stdio server exposes `echo`; a tools run can call it and the output
   appears in the transcript.
2. A server that hangs is killed after the timeout and the run continues with an
   `mcp.unavailable` event.
3. A tool named `write_file` from MCP is rejected/namespaced and cannot overwrite
   files through the built-in path.
4. No network is available to the server unless explicitly allowed.

## External agent adapters

`CommandBackend` already runs a trusted CLI with the prompt at
`/workspace/.aip/prompt.txt` and captures the workspace diff. An adapter is a
preset, not a new execution model:

| Adapter | Command | Notes |
|---|---|---|
| `claude-code` | `claude -p "$(cat .aip/prompt.txt)" --permission-mode acceptEdits` | requires the CLI in the sandbox image |
| `deepseek` | provider backend | no adapter needed (Messages API) |

### Requirements

- prompt file + task context are the same as for the tools backend;
- test command runs after the agent, as today;
- proof/design/secret gates are unchanged;
- model/version is recorded in the run;
- no API keys in the prompt; keys come from the sandbox env allowlist.

### Acceptance

1. The same task produces a diff with both `command` and `claude-code` adapters.
2. The proof/base guard and approval flow behave identically.
3. A missing CLI fails with `backend_unavailable`, not a silent empty diff.

Implemented preset: `AIP_AGENT_ADAPTER=claude-code` selects
`claude -p "$(cat /workspace/.aip/prompt.txt)" --permission-mode acceptEdits`.
The CLI is not in the default sandbox image yet, so this preset is unverified at
runtime; without the binary the run fails at the command step.

## Phases

1. MCP stdio client + fake server test (done 2026-10-26).
2. Tool provider integration in both loops + namespacing tests.
3. Sandbox execution + egress policy (Docker `StartProcess` done; wire MCP through it).
4. Project-level MCP config + UI (read-only list, admin enable).
5. Claude Code adapter preset + sandbox image variant.
6. Metrics: MCP call count/latency/failures.

## Risks

- MCP servers are arbitrary code: sandbox + trusted config are mandatory.
- Tool output can leak secrets: scan before persisting.
- Extra latency: per-call timeout and a per-run call budget.
- Adapter drift: contract tests run both backends against the same fake repo.
