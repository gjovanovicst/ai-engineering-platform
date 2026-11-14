# Kubernetes sandbox live drill

Status: **passed** on 2026-11-11 against a single-node kind cluster.

## Command

```bash
KIND_BIN=/path/to/kind scripts/k8s-sandbox-drill.sh
# keeps the cluster for inspection:
KEEP=1 KIND_BIN=/path/to/kind scripts/k8s-sandbox-drill.sh
```

The script creates `aip-sandbox`, loads `alpine:latest`, runs the opt-in
`TestKubernetesLiveDrill`, and deletes the cluster unless `KEEP=1`.

## Verified

```
=== RUN   TestKubernetesLiveDrill
--- PASS: TestKubernetesLiveDrill (4.61s)
PASS
```

The test proves, against the real API server:

- a Job is created and its pod becomes ready
- `Exec` returns `k8s-ok`
- `/etc` is **read-only** (write probe reports `readonly`)
- `/workspace` is **writable** (`workspace-ok`)
- `StartProcess` runs `cat`, accepts stdin, returns stdout and exits
- the Job and NetworkPolicy are removed by cleanup

The unit tests additionally assert the seccomp profile, dropped capabilities,
bounded emptyDir volumes and the network policy spec.

## Limits

- kind is a single-node development cluster, not a production CNI/OS
- NetworkPolicy enforcement was **not** exercised with a live egress attempt
- the drill uses `alpine`, not the full agent image
- no MCP server was run; `cat` proves the stdio process contract
- no multi-tenant or resource-pressure test
