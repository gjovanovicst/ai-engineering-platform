# Real Git rebase conflict in the merge queue

Status: **passed** on 2026-11-11.

The ten-PR e2e uses a fake GitLab API, so it proves ordering but not a real
conflict. This test closes that gap with an actual Git repository.

## Setup

`TestProcessAllHandlesRealGitRebaseConflict` in
`apps/api/internal/app/mergequeue/mergequeue_git_test.go`:

1. creates a bare origin and pushes an initial `main`
2. creates two PR branches that change the **same line** of `README.md`
3. merges PR 1 through the queue
4. lets the queue rebase PR 2 onto the new `main`

## Verified

```
PR 1: rebased + merged
PR 2: rebase -> CONFLICT (content): Merge conflict in README.md
queue status: merged, failed
merge of PR 2: not attempted
detail: rebase: git rebase origin/main ... CONFLICT ...
```

The test uses the real `git` CLI, not a fake rebaser. It also asserts that the
second PR is not merged after a failed rebase.

Run:

```bash
cd apps/api
go test ./internal/app/mergequeue -run TestProcessAllHandlesRealGitRebaseConflict -v
```

## What this proves and what it does not

Proves:

- the queue calls rebase before merge
- a real content conflict is surfaced as a failed queue item
- a failed rebase prevents the merge

Does not prove:

- the GitHub `update-branch` API returns the same error shape for every
  protected-branch rule (the GitHub adapter test covers the HTTP call)
- GitLab rebase support; GitLab does not implement `RebasePullRequest` in this
  adapter yet
- a real GitHub App / protected branch end-to-end run
