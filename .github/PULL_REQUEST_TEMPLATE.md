## What

<!-- What changed and where? -->

## Why

<!-- The problem, linked issue, or user impact. -->

## How tested

<!-- Commands, tests, screenshots, live evidence. -->

```text
make check
```

## Risk and rollback

Risk: <!-- low | medium | high — what can break -->

Rollback: <!-- revert, flag, migration, restore path -->

## Screenshot

<!-- Required for UI changes. Remove if not applicable. -->

## Checklist

- [ ] `make check` passes
- [ ] tests cover the change (or explain why not)
- [ ] no secrets, `.env`, keys or customer data
- [ ] docs/OpenAPI updated when behaviour changes
- [ ] `CHANGELOG.md` updated for `feat`/`fix`/`perf` (or `[skip-changelog]` in the title)
- [ ] breaking: yes/no
