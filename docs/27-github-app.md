# GitHub App setup

The platform works with a personal access token, but a GitHub App is the
production path: short-lived installation tokens, `checks:read`, and one app-wide
webhook instead of per-repo hooks.

## Why

| Problem with a PAT | GitHub App |
|---|---|
| `checks:read` often missing → checks sync falls back to Actions jobs | app tokens have the permissions granted at creation |
| webhook URL is per repository and must be edited by hand | one app webhook URL covers every installed repo |
| long-lived token in `.env` | installation token, cached and refreshed |

Checks sync (worker, every minute) already works without a webhook. The app makes
it lower-latency and removes the PAT scope problem.

## Create the app (one manual step)

1. GitHub → **Settings → Developer settings → GitHub Apps → New GitHub App**.
2. Name: `AI Engineering Platform` (any).
3. **Homepage URL**: the platform web URL (`AIP_WEB_URL`).
4. **Webhook URL**:
   `https://<public-host>/webhooks/github-app`
   (for local demos, the current quick tunnel).
5. **Webhook secret**: a random value; same as `AIP_GITHUB_APP_WEBHOOK_SECRET`.
6. **Repository permissions**:
   - Contents: read & write (clone + push)
   - Pull requests: read & write
   - Checks: read
   - Metadata: read
7. **Subscribe to events**: `Check run`, `Pull request`, `Pull request review`.
8. Create, then **Generate a private key** (`.pem`) and **Install** the app on the
   target repositories.

## Configure the platform

```bash
AIP_GITHUB_APP_ID=123456
AIP_GITHUB_APP_KEY_PATH=/secrets/github-app.pem
AIP_GITHUB_APP_INSTALL=12345678        # optional; first installation is used
AIP_GITHUB_APP_WEBHOOK_SECRET=<webhook secret>
```

Restart the API and worker. The worker uses the app token for PRs, pushes and
checks polling. The API accepts app webhooks at `POST /webhooks/github-app` and
resolves the repository by `repository.full_name`; check runs and merged-PR
events are handled exactly like per-repo webhooks.

## Verify

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST \
  https://<public-host>/webhooks/github-app \
  -H 'X-GitHub-Event: ping' -H 'Content-Type: application/json' -d '{}'
```

A `401` means the signature/secret is wrong; `404` means the app secret env is
missing. After a real PR, `test_results` should gain rows without waiting for the
poll interval.

## Boundaries

- Only the GitHub App webhook route is app-wide; GitLab/Gitea/Bitbucket keep
  per-repo webhooks.
- The app must be installed on each repository; the platform does not auto-install.
- App secrets live in the database only for providers; GitHub App credentials are
  environment configuration today.
