# Builds the API, worker and CLI in one image.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -o /out/aipd ./cmd/aipd \
 && CGO_ENABLED=0 go build -o /out/aip-worker ./cmd/aip-worker \
 && CGO_ENABLED=0 go build -o /out/aipctl ./cmd/aipctl \
 && CGO_ENABLED=0 go build -o /out/aip-kms-dev ./cmd/aip-kms-dev

FROM gcr.io/projectsigstore/cosign:v2.4.1 AS cosign

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git docker-cli tzdata
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign
COPY --from=build /out/aipd /usr/local/bin/aipd
COPY --from=build /out/aip-worker /usr/local/bin/aip-worker
COPY --from=build /out/aipctl /usr/local/bin/aipctl
COPY --from=build /out/aip-kms-dev /usr/local/bin/aip-kms-dev
ENTRYPOINT ["/usr/local/bin/aipd"]
