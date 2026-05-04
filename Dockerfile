# syntax=docker/dockerfile:1.6

ARG VERSION=dev
ARG REVISION=unknown

FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/spa/dist ./internal/spa/dist
ARG VERSION
ARG REVISION
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.cairnVersion=${VERSION} -X main.cairnRevision=${REVISION}" \
    -o /cairn ./cmd/cairn

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION
ARG REVISION

LABEL org.opencontainers.image.title="Cairn"
LABEL org.opencontainers.image.description="Self-hosted monitoring, incident management, and status pages"
LABEL org.opencontainers.image.url="https://cairn.arne.sh"
LABEL org.opencontainers.image.source="https://github.com/TRC-Loop/cairn"
LABEL org.opencontainers.image.documentation="https://cairn.arne.sh"
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"
LABEL org.opencontainers.image.vendor="TRC-Loop"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${REVISION}"

ENV CAIRN_DB_PATH=/data/cairn.db \
    CAIRN_LISTEN_ADDR=:8080 \
    CAIRN_BEHIND_TLS=false

COPY --from=builder /cairn /cairn

USER nonroot:nonroot
EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/cairn", "healthcheck"]

ENTRYPOINT ["/cairn"]
