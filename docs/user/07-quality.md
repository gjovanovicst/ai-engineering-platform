# Quality gates

Quality is enforced by evidence, not by asking the agent to "be careful".

## The test ladder

| Layer | What it proves |
|---|---|
| Unit | Logic and edge cases in isolation |
| Integration | Boundaries: database, queue, object storage |
| End-to-end | The user flow works through the real surfaces |
| Visual | The UI matches the approved design baseline |
| Security | No new secrets, vulnerable dependencies or forbidden changes |

The project's test strategy defines which layers are required. The e2e command runs in the sandbox after unit tests; a failure blocks the PR.

## Acceptance evidence

Every task has acceptance criteria. A development run must produce an `AIP_PROOF` artifact that maps evidence to each criterion:

```
criterion: user can sign in with SSO
evidence: e2e/sso.spec.ts > signs in with SAML
```

The platform validates that the referenced test or file exists in the diff. A proof that points to a missing test fails the run.

## Design checks

For UI changes the platform can compare a rendered screen against the project's baseline:

- layout, spacing and typography
- component usage from the design system
- accessibility checks
- visual diff threshold

A visual difference does not approve or reject itself. It produces evidence for the human reviewer.

## Diff policy

Project rules can block changes before review:

- forbidden paths (secrets, generated files, infrastructure)
- maximum diff size
- dependency changes that need an ADR
- missing migration for a schema change
- new network calls in the sandbox

## Review findings

A review run reports findings with severity. Findings are not automatically blocking unless the project says so; the PR approval remains the human gate.

## What to do when a gate fails

1. Read the failing check, not just the summary.
2. Reproduce locally with the same command.
3. Fix the cause, not the assertion.
4. Re-run the check or the agent run.
5. Record an override only when a human accepts the risk; the override is audited.

## Next

- [Pull requests](pull-requests.html)
- [Troubleshooting](troubleshooting.html)
