# Agent sandbox hardening drill

Status: **run** on 2026-11-05.

The Docker sandbox already creates containers with `--cap-drop ALL`,
`--security-opt no-new-privileges`, `--read-only`, tmpfs workspaces and
`--network none`. `scripts/sandbox-hardening-drill.sh` proves those flags.

## Verified run

```
root_write=1
workspace_write=0
etc_mkdir=1
mount=1
CapEff: 0000000000000000
network=1
sandbox hardening drill ok
```

- the root filesystem and `/etc` are not writable
- `/workspace` (tmpfs) is writable, as intended
- no capabilities, no mounts, no network egress
- `workspace_write=1` in the first attempt was a drill bug (tmpfs not owned by
  uid 10001), not a sandbox failure; the corrected drill passes

## Not covered

- seccomp/AppArmor profile strength (Docker default seccomp only)
- kernel exploits / container escape
- Kubernetes sandbox driver
- egress allowlists for tool backends that need network (not implemented; `AIP_SANDBOX_EGRESS_ALLOW` now fails startup instead of being silently ignored)


Kubernetes equivalent: [71-kubernetes-sandbox.md](71-kubernetes-sandbox.md). It is unit-tested but has not been drilled on a live cluster.
