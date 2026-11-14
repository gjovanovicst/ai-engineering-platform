# Pull requests and merges

The platform opens a pull request when a development run is ready. The PR is the hand-off between the agent's diff and the team's repository rules.

![Pull requests and checks](../../assets/product-pull-requests.png)

*Every PR shows its provider, number, status and required checks. The merge queue serialises landing.*

## What is checked

Before a PR can merge:

- required checks are green
- the test gate passed and coverage did not drop beyond the project threshold
- design checks passed for UI changes
- acceptance evidence maps to every criterion
- the required number of humans approved
- the merge window allows landing

A missing check is not a warning. The merge button stays unavailable.

## Reviewing a PR

1. Open the PR from the run or the project's **Delivery → Pull requests** view.
2. Read the diff and the run evidence.
3. Leave comments or request changes.
4. Approve when the change is correct.

Approvals are distinct users. The author of the run cannot count as the approver when the project requires an independent review.

## Merge queue

The merge queue lands approved changes in order. It exists to prevent two green PRs from breaking each other at the last moment.

- A PR enters the queue after approval.
- The queue rebases or re-runs required checks as configured.
- If a check fails, the PR leaves the queue and returns to the author with the reason.

## Merge windows

Admins can restrict landing to a time window. Outside the window, approved PRs wait; the queue shows the next allowed time.

## Failed checks

A failed check keeps the PR open. Fix the cause, push a new commit and let the checks run again. Do not weaken a check to make it pass; change the code or the test.

## Provider differences

GitHub and GitLab are supported. Both can rebase a PR/MR before merge; GitLab's
rebase is asynchronous and the queue waits for the conflict status before it
merges. The provider is recorded on the repository and on each PR. Branch
protection remains the final authority in the Git provider; the platform does
not bypass it.

## Next

- [Quality gates](quality.html)
- [Analytics](analytics.html)
