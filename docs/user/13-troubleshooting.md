# Troubleshooting

Start with the run view and the event stream. Most failures have a specific event before the generic error.

## Sign-in

| Symptom | Check |
|---|---|
| Redirect loop | Cookies blocked, wrong `AIP_WEB_URL`, or clock skew |
| SSO button missing | OIDC/SAML not configured or enabled for the workspace |
| 401 after login | Session expired or the API JWT key changed |
| 403 on a project | Workspace role is insufficient or project access is narrower |
| 401/403/404 from GitHub or GitLab | Token scope, SSO authorisation or repo access — see [Git providers](git-providers.html#troubleshooting) |

## A run will not start

- The task is still **Draft**; complete the definition of ready.
- The repository is unreachable; check the repository credential, the GitHub App
  installation or the GitLab connection ([Git providers](git-providers.html)).
- The project has no allowed agent/model.
- A previous active run exists for the task.

## A run failed

Open the run and read the last events:

- **clone failed** — repository access or branch name
- **provider error** — credential, quota or model name
- **budget exceeded** — raise the budget or split the task
- **missing proof** — acceptance evidence does not map to a real test
- **check failed** — read the failing test/check output, do not retry blindly

Retry creates a new run; it does not erase the failed one.

## A PR is blocked

- A required check is red. Open the check, not just the badge.
- The approval count is not met. Approvals must be distinct users.
- The merge window is closed.
- The merge queue found a conflict or a failed re-check.

## A restore failed

- The target is before the base backup or after the last archived WAL.
- The object store credential expired.
- The scratch volume still contains data; the runner cleans it before fetch.

The request keeps the failure reason; do not delete it before recording the incident.

## Getting help

Include the workspace, project, task or run ID, the timestamp and the exact error. Never include provider keys, tokens or customer data.

## Next

- [Runs and approvals](runs-and-approvals.html)
- [Backups and recovery](backups-recovery.html)
- [API reference](../api.html)
