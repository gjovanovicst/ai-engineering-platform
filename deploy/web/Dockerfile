# Builds the Next.js web app as a production image.
#
# The API URL is a build argument because Next.js inlines NEXT_PUBLIC_* at build
# time. For a docker-compose quickstart the default points at the API published
# on the host (http://localhost:18080); a deployment passes its own URL.
FROM node:22-alpine AS deps
WORKDIR /app
RUN corepack enable
COPY apps/web/package.json apps/web/pnpm-lock.yaml apps/web/pnpm-workspace.yaml ./
# --ignore-scripts: the Next build uses SWC, not the esbuild/postinstall binaries;
# skipping dependency scripts also keeps the layer deterministic in CI.
RUN pnpm install --frozen-lockfile --ignore-scripts

FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable
COPY --from=deps /app/node_modules ./node_modules
COPY apps/web/ ./
ARG NEXT_PUBLIC_API_URL=http://localhost:18080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
COPY --from=build --chown=node:node /app/.next ./.next
COPY --from=build --chown=node:node /app/public ./public
COPY --from=build --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/package.json ./package.json
COPY --from=build --chown=node:node /app/next.config.mjs ./next.config.mjs
EXPOSE 3000
USER node
CMD ["node_modules/.bin/next", "start", "-p", "3000"]
