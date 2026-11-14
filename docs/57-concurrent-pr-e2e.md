# Ten concurrent PR e2e

Status: **passed** on 2026-11-05.

`E2E_CONCURRENT=1 N=10 scripts/e2e.sh` starts the fake-GitLab stack, creates ten
tasks, drives plan approvals, development, PR approvals and merges, then asserts
ten distinct merged PRs.

## Verified run

```
[e2e] concurrent PRs (10)
started 10 tasks
pull requests: [4, 10, 3, 9, 7, 2, 1, 5, 6, 8]
merged 10 PRs; distinct: 10
```

The worker concurrency gate (2 per org) processes them in waves; all ten PRs are
open concurrently before the merges and each gets a unique GitLab `iid`.

## Bugs found by the harness

- the frozen context hashed `.git/` (concurrent pushes changed the hash and
  produced `context_changed`); `.git/` is now excluded from `tarFiles` and
  `hashWorkspace`
- `visual/*` run artifacts violated `run_artifacts_kind_check`; migration 0073
  allows that prefix and stores PNGs

## Limits

- fake GitLab does not apply real merges/rebase conflicts; it exercises the
  platform's queue/approval/idempotency, not remote Git conflict handling
- agent output is the fake command (same file on every branch)
- the old single-PR e2e remains the default (`E2E_CONCURRENT=0`)


Real Git conflict coverage: [69-real-git-conflict.md](69-real-git-conflict.md). The ten-PR run still uses a fake GitLab API; the conflict test uses actual git.
