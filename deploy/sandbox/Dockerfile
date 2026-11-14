# Sandbox image for agent runs: Go, Node, git, ripgrep, jq.
# Built once and pinned by digest in production (AIP_SANDBOX_IMAGE).
FROM golang:1.25-alpine AS go

FROM node:22-alpine

RUN apk add --no-cache bash ca-certificates curl git jq ripgrep tini

# Go toolchain (agents build with CGO_ENABLED=0, so no gcc is needed).
COPY --from=go /usr/local/go /usr/local/go

RUN npm install -g pnpm@9.15.0 && npm cache clean --force

# Optional external agent CLI; empty keeps the default image unchanged.
ARG CLAUDE_CODE_VERSION=
RUN if [ -n "$CLAUDE_CODE_VERSION" ]; then \
      npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" && npm cache clean --force; \
    fi

ENV PATH="/usr/local/go/bin:${PATH}" \
    GOTOOLCHAIN=local \
    GOMODCACHE=/cache/go \
    GOCACHE=/cache/go-build \
    PNPM_HOME=/cache/pnpm \
    CI=1 \
    NO_COLOR=1

RUN adduser -D -u 10001 -s /bin/bash agent \
    && mkdir -p /workspace /cache/go /cache/go-build /cache/pnpm \
    && chown -R agent:agent /workspace /cache

USER agent
WORKDIR /workspace

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["sh", "-c", "sleep infinity"]
