# Sandbox egress policy

Status: **fail-closed** on 2026-11-11. A domain allowlist is **not implemented**.

## Current behaviour

- The Docker and Kubernetes sandbox drivers run with network disabled unless a
  run explicitly enables it.
- `AIP_SANDBOX_EGRESS_ALLOW` used to exist in `.env.example` but was not read by
  any code. A deployment could set it and still get an unrestricted network.
- Setting it now makes `config.Load` fail at startup:

```text
AIP_SANDBOX_EGRESS_ALLOW is not implemented; remove it (the sandbox network is disabled)
```

This is intentional: an unimplemented security control must not be silently
ignored.

## Why not just allow the listed domains?

A domain allowlist needs one of:

1. an HTTP CONNECT proxy that terminates and filters traffic, with `HTTP_PROXY`
   set inside the sandbox; or
2. a CNI that supports FQDN policies; or
3. a DNS-aware firewall that pins resolved addresses and blocks rebinding.

Docker's `--network none` is closed, but it has no domain-aware mode. A Linux
NetworkPolicy on Kubernetes is CIDR/port based; resolving a domain once is
brittle and does not handle CDN changes or DNS rebinding.

## Next implementation

The safe design is a per-run egress proxy:

- sandbox on an internal network with no default route
- proxy on the internal network plus an external network
- allowlist enforced on CONNECT host and SNI
- DNS only through the proxy
- proxy logs and byte limits per run

That needs a proxy image and a live egress test. Until then, the only supported
mode is offline.

## Host-side module fetch (implemented)

The sandbox stays offline; the **worker host** can fetch Go modules on the
agent's behalf. The development agent calls the `fetch_module` tool with a
module path and version:

```json
{"module": "golang.org/x/crypto", "version": "v0.36.0"}
```

What happens:

1. The worker validates the coordinates (module path and semver; no flags, URLs
   or path traversal).
2. It runs `go mod download module@version` on the host, through
   `AIP_DEPS_GOPROXY` (default `https://proxy.golang.org,direct`), with the Go
   checksum database enforcing integrity.
3. It packs the resulting module cache and extracts it into the sandbox at
   `/cache/gomod`; the sandbox runs with `GOPROXY=off`, `GOSUMDB=off` and
   `GOFLAGS=-mod=mod`, so `go mod tidy` and the tests resolve from that cache.
4. It emits a `dependency.fetch` run event with the outcome.

The sandbox never gets a network interface, and the agent cannot fetch arbitrary
URLs: only Go module coordinates, only through the configured proxy, only into
the offline cache. A failure is reported to the model and the run can classify
it as `dependency_missing` if the module is still absent.

This is not an egress allowlist: the proxy traffic happens on the worker host,
outside the sandbox. The per-run egress proxy above remains the design for tools
that need arbitrary network access.

## Host-side npm fetch (implemented)

The web follows the same rule: the sandbox stays offline, and the **worker
host** installs the repository's locked dependencies before the workspace is
archived. When a repository has `pnpm-lock.yaml` (at the root or in `apps/web/`)
and `AIP_DEPS_NPM` is on (the default), the cloner runs

```sh
pnpm install --frozen-lockfile --ignore-scripts
```

on the host, in the directory with the lockfile, and the resulting
`node_modules` becomes part of the workspace archive the sandbox extracts. The
agent can then run `pnpm test`, `next build` or `eslint` offline.

The install is best-effort: a failure is logged, the run continues without
`node_modules`, and the test gate reports the failure. A `dependency.fetch` run
event (`manager: npm`) records that the workspace carries web dependencies.
`AIP_PNPM_BIN` selects the pnpm binary (default `pnpm`); `AIP_DEPS_NPM=false`
turns the preparation off.
