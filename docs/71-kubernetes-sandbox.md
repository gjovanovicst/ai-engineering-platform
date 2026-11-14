# Kubernetes sandbox hardening and MCP process

Status: **implemented, unit-tested and live-drilled** on 2026-11-11 (kind; see
[72-kubernetes-live-drill.md](72-kubernetes-live-drill.md)).

## Job hardening

Each run creates one Kubernetes Job with:

- `runAsNonRoot`, fixed UID/GID 10001
- all Linux capabilities dropped
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `seccompProfile: RuntimeDefault` on pod and container
- writable `emptyDir` mounts with size limits:
  - `/workspace` 512Mi
  - `/tmp` 256Mi
  - `/home/agent` 64Mi
  - `/cache` 512Mi
- CPU and memory requests/limits from the run spec
- one Job per run, `backoffLimit: 0`
- when `NetworkDisabled` is set, an ingress+egress `NetworkPolicy` with an
  impossible port closes both directions

## MCP process

`Kubernetes.StartProcess` now uses `pods/exec` over SPDY and exposes the same
`Process` contract as Docker:

- stdin/stdout/stderr pipes
- `Wait` returns the exec result
- `Kill` cancels the stream and closes stdin
- environment variables are passed through an `env KEY=VALUE ...` wrapper,
  because Kubernetes exec has no `env` field

## Tests

`apps/api/internal/agent/sandbox/kubernetes_test.go` asserts the Job spec
(read-only root, seccomp, four bounded volumes, no retries), the env command
ordering and the config guard for `StartProcess`.

## Limits

- the live drill is a single-node kind cluster, not a production CNI/OS
- `NetworkPolicy` enforcement depends on the CNI
- no domain-level egress allowlist; network is either closed or open
- no AppArmor profile
- no MCP process drill inside a real pod
- `Kill` closes the exec stream; it does not send a Unix signal to the process
