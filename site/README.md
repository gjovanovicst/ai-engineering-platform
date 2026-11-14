# GitHub Pages site

Static product site plus the user documentation portal. No runtime framework.

## Local preview

```bash
make docs-site          # regenerate docs/ + API reference (needs PyYAML)
cd site
python3 -m http.server 4173
# open http://127.0.0.1:4173
```

- Product site: `http://127.0.0.1:4173/`
- User guide: `http://127.0.0.1:4173/docs/`
- API reference: `http://127.0.0.1:4173/docs/api.html`

## Documentation

Source of truth:

- `docs/user/*.md` — task-oriented user guide (setup, projects, tasks, runs,
  approvals, security, administration)
- `api/openapi.yaml` — every endpoint and schema in the API reference
- `api/events.md` — SSE event catalog

`make docs-site` renders them into `site/docs/`. Do not edit generated HTML by
hand.

## Deploy

1. Push to `main`.
2. Repository **Settings → Pages → Build and deployment → Source: GitHub Actions**.
3. Set the repository variable `PAGES_ENABLED=true`.
4. The `pages` workflow builds the docs, uploads `site/` and deploys it.
   Without the variable the workflow is skipped; Pages must also be enabled in
   Settings → Pages with source GitHub Actions.

The workflow also runs when `docs/user/`, `api/openapi.yaml`, `api/events.md` or
the builder changes.

## Content rule

The **Status** section intentionally says what is not production-ready
(fencing, multi-region, sync RPO). Keep public copy aligned with
`docs/54-status.md`.
