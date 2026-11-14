# Contributing

Thanks for helping. This project is a governed execution layer for AI agents; the
same gates the product enforces apply to changes here.

## Before you start

- Read [docs/01-asdlc.md](docs/01-asdlc.md) for the process and gates.
- Read [docs/02-architecture.md](docs/02-architecture.md) for package boundaries.
- For API changes, read [docs/04-api.md](docs/04-api.md) and `api/openapi.yaml`.
- For UI changes, read [docs/06-design-system.md](docs/06-design-system.md) and
  [docs/19-a11y.md](docs/19-a11y.md).

## Development

```bash
cp .env.example .env
make keygen
docker compose up -d postgres redis minio mailhog
make dev
make check
```

`make check` is the minimum bar: formatting, vet, lint, Go tests, web typecheck
and lint, and the OpenAPI diff. If you touch a flow covered by `make e2e`, run it.

## Branches and commits

- Branch from `main`: `feat/<short-slug>`, `fix/<short-slug>`, `docs/<slug>`.
- Do not commit directly to `main`.
- Sign off every commit (Developer Certificate of Origin):

```bash
git commit -s -m "feat(tasks): ..."
```

- The sign-off certifies you have the right to submit the contribution under the
  project license (Apache-2.0). No separate CLA is required.
- Keep commits focused; a PR over ~400 changed lines needs a reason.

## Pull requests

Include the five sections from `.github/PULL_REQUEST_TEMPLATE.md`:

1. What
2. Why
3. How tested
4. Risk and rollback
5. Screenshot (UI changes)

State `breaking: yes|no` if the OpenAPI contract changes. Agents do not approve
or merge their own PRs.

## Code rules

- Go: `context.Context` first, no I/O in `internal/domain`, errors wrapped, SQL
  through sqlc, no `SELECT *`, no panics outside `main`.
- TypeScript: strict, no `any`, no hex colors in components, all lists have
  loading/empty/error/ready states.
- Never weaken a test to make it pass.
- Do not add a dependency without an ADR.

## Security and secrets

- Never commit `.env`, keys, tokens, database dumps or customer data.
- Report vulnerabilities privately: see [SECURITY.md](SECURITY.md).
- Run `make oss-audit` before opening a PR that adds configuration or docs.

## Documentation

- Public docs: user guide, API, architecture, security model.
- Internal docs: drill reports, topology, incident logs. Do not publish these.
- The public documentation site is generated with `make docs-site`.

## License

By contributing, you agree that your contribution is licensed under
[Apache-2.0](LICENSE).
