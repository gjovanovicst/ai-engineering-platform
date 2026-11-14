SHELL := bash
.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
API_DIR := apps/api
BIN := $(CURDIR)/bin

# local cache (works inside a sandbox; override via env)
export GOPATH ?= $(CURDIR)/.cache/gopath
export GOCACHE ?= $(CURDIR)/.cache/go-build
export GOMODCACHE ?= $(CURDIR)/.cache/gomod
export GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint

GOFILES := $(shell find $(API_DIR) -name '*.go' -not -path '*/.cache/*')

.PHONY: help build test test-integration lint fmt fmt-check vet generate oapi-check security check dev run-api run-worker sandbox-image e2e backup restore-check pilot-check loadtest loadtest-k6 \
        web-install web-dev web-check \
        migrate-up migrate-down migrate-status seed bootstrap keygen tools tools-check \
        compose-up compose-down clean

help: ## show available commands
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## build all three binaries
	cd $(API_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/aipd ./cmd/aipd
	cd $(API_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/aip-worker ./cmd/aip-worker
	cd $(API_DIR) && go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/aipctl ./cmd/aipctl

test: ## unit tests
	cd $(API_DIR) && go test -race -count=1 ./...

test-integration: ## integration tests (needs AIP_TEST_DB_* and a reachable database)
	cd $(API_DIR) && go test -race -count=1 -tags=integration ./...

lint: ## golangci-lint
	@command -v golangci-lint >/dev/null || { echo "golangci-lint is not installed (make tools)"; exit 1; }
	cd $(API_DIR) && golangci-lint run ./...

fmt: ## format Go code
	gofmt -w $(GOFILES)

fmt-check: ## check formatting
	@files="$$(gofmt -l $(GOFILES))"; if [ -n "$$files" ]; then echo "gofmt needed:"; echo "$$files"; exit 1; fi

generate: ## regenerate sqlc code and OpenAPI models
	cd $(API_DIR) && sqlc generate
	cd $(API_DIR) && oapi-codegen -config oapi-codegen.yaml ../../api/openapi.yaml

oapi-check: ## fail when generated OpenAPI models are stale
	@command -v oapi-codegen >/dev/null || { echo "oapi-codegen is not installed (make tools)"; exit 1; }
	cd $(API_DIR) && oapi-codegen -config oapi-codegen.yaml ../../api/openapi.yaml
	@git diff --exit-code -- $(API_DIR)/internal/httpapi/oapi \
		|| { echo "generated models are stale; run 'make generate'"; exit 1; }

vet: ## go vet
	cd $(API_DIR) && go vet ./...

oss-audit: ## check the working tree before publishing
	bash scripts/oss-audit.sh

k8s-drill: ## live Kubernetes sandbox drill on kind
	bash scripts/k8s-sandbox-drill.sh

docs-site: ## build the static user guide + API reference
	python3 scripts/build_user_docs.py

check: fmt-check vet lint test ## what CI runs before merge
	@echo "OK"

dev: compose-up ## start the database, the API and the worker
	@trap 'kill 0' EXIT; \
	$(MAKE) run-api & \
	$(MAKE) run-worker & \
	wait

run-api: ## run the API (:8080)
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipd

run-worker: ## run the worker
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aip-worker

migrate-up: ## apply migrations
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipctl migrate up

migrate-status: ## show migration status
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipctl migrate status

migrate-down: ## roll back the last migration
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipctl migrate down

seed: ## insert demo data
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipctl seed

bootstrap: ## first owner account + organization
	@set -a; [ -f .env ] && . ./.env; set +a; cd $(API_DIR) && go run ./cmd/aipctl bootstrap

keygen: ## generate an Ed25519 key for JWT
	@test -f jwt.pem || openssl genpkey -algorithm ed25519 -out jwt.pem
	@echo "jwt.pem is ready (gitignored)"

tools: ## install the CLI tools (goose, sqlc, oapi-codegen)
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

tools-check: ## verify the tools are available
	@for t in go gofmt golangci-lint sqlc oapi-codegen node pnpm docker; do \
		printf "%-14s" "$$t"; command -v $$t >/dev/null && echo ok || echo "MISSING"; \
	done

security: ## dependency scans (govulncheck in apps/api, pnpm audit in apps/web)
	cd apps/api && govulncheck ./...
	cd apps/web && pnpm audit --prod
	node scripts/check-js-licenses.mjs

loadtest-k6: ## k6 API load test (RUNS=50 VUS=10 K6=path/to/k6)
	RUNS=$${RUNS:-50} VUS=$${VUS:-10} AIP_BASE=$${AIP_BASE:-http://localhost:18080} 	  $${K6:-k6} run scripts/k6/loadtest.js

loadtest: ## N planning runs against a real worker (LOAD_N=10, LOAD_P95_LIMIT=120)
	LOAD_N=$${LOAD_N:-10} LOAD_P95_LIMIT=$${LOAD_P95_LIMIT:-120} bash scripts/loadtest.sh

pilot-check: ## repository-side pilot readiness (docs/17-pilot-checklist.md)
	sh scripts/pilot-check.sh

backup: ## logical backup to ./backups (AIP_DB_DSN required)
	BACKUP_DIR=$${BACKUP_DIR:-./backups} sh scripts/backup.sh

restore-check: ## restore the newest dump into a scratch DB (AIP_DB_DSN required)
	sh scripts/restore-check.sh $$(ls -t backups/aip-*.dump | head -1)

e2e: ## end-to-end: plan -> approval -> dev -> MR -> gate -> merge -> metrics
	bash scripts/e2e.sh

verify-image: ## verify a signed API image (IMAGE=..., AIP_COSIGN_IDENTITY_REGEXP=...)
	sh scripts/verify-image.sh $${IMAGE:?set IMAGE=registry/image@sha256:...}

sandbox-image: ## build the agent sandbox image (Go + Node + git + ripgrep)
	docker build --build-arg CLAUDE_CODE_VERSION=$${CLAUDE_CODE_VERSION:-} -t aip/sandbox:go1.25-node22 deploy/sandbox

compose-up: ## postgres, redis, minio, mailhog
	docker compose up -d postgres redis minio mailhog

web-install: ## install web dependencies (pnpm)
	cd apps/web && pnpm install

web-dev: ## run Next.js on :3000
	cd apps/web && pnpm dev

web-check: ## lint, type-check and test the web app
	cd apps/web && pnpm lint && pnpm tsc && pnpm test

compose-down: ## stop the containers
	docker compose down

clean: ## remove build artifacts
	rm -rf $(BIN) .cache coverage.out
