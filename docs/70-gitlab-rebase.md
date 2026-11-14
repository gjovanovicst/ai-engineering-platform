# GitLab merge-request rebase

Status: **implemented and unit-tested** on 2026-11-11.

## Behaviour

`GitLab.RebasePullRequest` calls GitLab's asynchronous API:

```text
PUT /api/v4/projects/{project}/merge_requests/{iid}/rebase
```

GitLab returns `202` immediately. The adapter then polls the MR until
`rebase_in_progress` is false and interprets the result:

- `has_conflicts: true` → error (`conflict`)
- `merge_status: cannot_be_merged` → error
- `detailed_merge_status: conflict` → error
- otherwise → success

The merge queue only calls `MergePullRequest` after the rebase succeeds.
A failed rebase is recorded as `failed` with the conflict detail and the MR is
not merged.

## Tests

`apps/api/internal/adapters/git/gitlab_test.go`:

- success: `202`, poll reports no conflict
- conflict: poll reports `has_conflicts` and `cannot_be_merged`
- HTTP failure: `409` from the rebase endpoint

## Limits

- this is a **unit test with an HTTP stub**, not GitLab.com or a live self-hosted
  GitLab
- GitLab's rebase is asynchronous; the adapter waits up to 30 seconds
- protected-branch policy is still enforced by GitLab, not by the adapter
- a real GitLab merge conflict e2e is not claimed
