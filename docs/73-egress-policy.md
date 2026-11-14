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
