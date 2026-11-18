# Quickstart

Run the platform locally, then register and start the first project.

## Prerequisites

- Docker with Compose v2 (the worker needs the Docker socket for the sandbox)
- Go 1.25+ and `make`
- Node.js 22 with pnpm 11 (`corepack enable`)

## 1. Configure

```sh
git clone https://github.com/gjovanovicst/ai-engineering-platform.git
cd ai-engineering-platform
cp .env.example .env
make keygen                 # writes jwt.pem (ignored by git)
```

Set at least one LLM provider in `.env` (any OpenAI-compatible endpoint):

```sh
AIP_PROVIDER_OPENAI_BASE_URL=https://api.deepseek.com
AIP_PROVIDER_OPENAI_KEY=sk-...
```

## 2. Infrastructure and schema

```sh
docker compose up -d postgres redis minio mailhog keycloak
make migrate-up
```

The compose Postgres publishes `5432` and the `.env.example` DSNs already point
at it.

## 3. Sandbox image

```sh
make sandbox-image
```

The agent runs code in that image; without it, runs fail before the first step.

## 4. Run the services

Either with compose:

```sh
docker compose up -d api worker web
```

or with the local toolchain (three terminals):

```sh
make run-api
make run-worker
make web-dev
```

The web is at <http://localhost:3000>, the API at
<http://localhost:18080>. The docs site is `make docs-site` (or
`site/index.html`).

## 5. First project

Either seed the demo data:

```sh
make seed
```

That creates the org **Acme**, the owner `demo@acme.io` (password from
`AIP_SEED_PASSWORD`, default `correct horse battery` — local only), the **Demo**
project and three ready tasks. It is idempotent, so running it again is safe.

... or register yourself:

1. Open <http://localhost:3000/register> and create the owner account.
2. Create an organization and a project in the web UI.
3. Connect the repository the agents should work in (a fine-grained GitHub PAT:
   Contents RW, Pull requests RW, Workflows RW, Actions: Read).
4. Add a task with acceptance criteria, start a planning run, approve the plan,
   and follow the run to the pull request.

The task-to-PR path is described in [docs/01-asdlc.md](01-asdlc.md); per-repo
git credentials are in [docs/user/15-git-providers.md](user/15-git-providers.md).

## Troubleshooting

- `readyz` fails: check the Postgres DSN in `.env` and that migrations ran.
- A run fails before the first step: build the sandbox image, or point
  `AIP_SANDBOX_IMAGE` at the published sandbox image.
- Agent runs stop immediately: the provider key/base URL is wrong or empty.
- Web shows API errors: the web image was built with the wrong
  `NEXT_PUBLIC_API_URL` (Next inlines it at build time).
