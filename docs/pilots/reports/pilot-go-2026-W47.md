# pilot-go — 2026-W47

The first weekly pilot run on `gjovanovicst/pilot-go`.

## The run

| Step | Run | Status |
|---|---|---|
| Planning | `bcbae006` | succeeded (the plan was approved by the human) |
| Development | `6aaea440` | succeeded |
| Review | `4d6aed8c` | succeeded (findings returned) |

The task: **T-1 — Add a health endpoint** (from `docs/tasks.md`).

## The outcome

The chain ran end to end: the plan was produced and approved, the development
run wrote the change, the review run read it and returned findings. The task
entered `review`.

The PR was **not** opened: the sandbox could not push the branch, so the task
has no `pr_number`. The next run should prove the push + the PR creation.

## The friction

1. **The login rate limit** (10/min) blocked the scripted setup twice; the pilot
   needs a longer window or a token cache.
2. **`POST /projects` rejects `slug`** — the field is not in `ProjectInput`
   (the slug is derived from the name); the error says "invalid JSON body",
   which misleads. A 400 should name the unknown field.
3. **`PUT /test-command` takes `{"command"}`, not `{"test_command"}`** — the
   response returns `test_command`, so the round trip is asymmetric.
4. **No `PATCH /projects`** — the test command needs its own route (it has one)
   but a general update does not exist; the setup script had to guess.
5. **The webhook is not created** by the platform — the human must add it in
   GitHub; the repo registration stores the secret but cannot install it.

## The bugs (from the review)

1. `internal/httpapi/health.go`: the `json.Marshal` error branch is unreachable
   for `healthResponse{Status string}`, and it reports via `http.Error`, which
   sets `Content-Type: text/plain` and appends a newline — the JSON error body
   would be mislabeled. Either drop the branch or use `json.NewEncoder`.

## The next week

- Push the branch and open the PR (the missing link).
- Add the webhook by hand once, then record the URL in the playbook.
- Split the friction items into small stories.
