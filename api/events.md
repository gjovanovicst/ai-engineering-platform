# API · SSE event catalog

`GET /api/v1/orgs/{org}/runs/{run}/events`

```
Content-Type: text/event-stream
Cache-Control: no-cache
X-Accel-Buffering: no
```

## Protocol

```
id: 41
event: phase.completed
data: {"run_id":"7c9e","phase":"implement","duration_ms":142300,"ts":"2026-10-21T18:22:09Z"}

: ping

```

- `id` = `run_events.seq` (increases monotonically per run). On reconnect the client sends
  `Last-Event-ID`; the server first backfills from the database, then subscribes to Redis.
- An unknown event type is **ignored** (forward compatibility).
- Heartbeat `: ping` every 15s. The server closes the connection after 1h; the client reconnects.
- `retry: 3000` is sent at the start of the stream.
- If `Last-Event-ID` does not exist in the database (archived), the server sends
  `event: stream.reset` with `{"from_seq": N}` and continues from the oldest available.

## Types

| `event` | When | `data` (key fields) |
|---|---|---|
| `run.queued` | run created | `run_id`, `agent_type`, `model` |
| `run.started` | worker picked it up | `started_at` |
| `phase.started` | entering a phase | `phase`, `attempt` |
| `phase.completed` | leaving a phase | `phase`, `duration_ms`, `meta` |
| `phase.failed` | phase failed | `phase`, `code`, `detail` |
| `llm.started` | before the LLM call | `model`, `prompt_tokens_est` |
| `llm.chunk` | token stream | `text` (delta), `index` |
| `llm.completed` | end of the call | `tokens_in`, `tokens_out`, `cost_micros`, `finish_reason` |
| `tool.call` | tool call | `tool`, `args_digest`, `permitted` |
| `tool.result` | result | `tool`, `ok`, `duration_ms`, `bytes` |
| `file.changed` | FS change in the sandbox | `path`, `change` (`added`/`modified`/`deleted`) |
| `plan.ready` | plan ready for approval | `artifact_id`, `sha256`, `summary` |
| `approval.requested` | waiting for a human | `approval_id`, `type` |
| `approval.decided` | decision | `approval_id`, `status`, `decided_by` |
| `test.started` / `test.completed` | test run | `suite`, `passed`, `failed`, `coverage_pct` |
| `diff.ready` | diff finalized | `artifact_id`, `files`, `additions`, `deletions` |
| `usage.updated` | after every LLM call | `tokens_in`, `tokens_out`, `cost_micros`, `budget_pct` |
| `run.succeeded` | end | `duration_ms`, `head_commit`, `pr_url?` |
| `run.failed` | end | `code`, `detail`, `retryable` |
| `run.canceled` | cancellation | `requested_by` |
| `stream.reset` | gap in the history | `from_seq` |

## Ordering and guarantees

- Events of the same run are **ordered** by `seq`; the client must not assume they arrive
  exactly once — a duplicate is skipped by `id`.
- `llm.chunk` is **best-effort** (it may be lost on reconnect); the full text is in the
  `log` artifact. After a reconnect the UI loads the last N events and continues.
- No `llm.chunk` carries prompt/system messages (only the model output).

## Minimal client pseudocode

```ts
const es = new EventSource(url, { withCredentials: false });
es.onmessage = (m) => apply(JSON.parse(m.data));
es.addEventListener('run.failed', (m) => showError(JSON.parse(m.data)));
es.onerror = () => { /* EventSource reconnects by itself; on 401 → refresh token */ };
```

SSE authorization goes through the `Authorization` header, so in practice `fetch` +
`ReadableStream` is used (EventSource does not send headers). Reference implementation:
`web/src/lib/sse.ts`.
