# syntax=docker/dockerfile:1.7

FROM node:26-alpine AS frontend-builder

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json frontend/.npmrc ./
RUN npm ci

COPY frontend/ ./
ENV NUXT_PUBLIC_API_BASE_URL=
RUN npm run generate

FROM golang:1.26.4-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
RUN set -eux; \
    arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${arch}" \
      go build -trimpath -ldflags="-s -w" -o "/out/promptgate" "."

FROM debian:bookworm-slim AS runtime

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates tzdata; \
    rm -rf /var/lib/apt/lists/*; \
    useradd --system --uid 10001 --home /nonexistent --shell /usr/sbin/nologin promptgate

WORKDIR /app

COPY --from=builder /out/ /app/
COPY --from=frontend-builder /src/frontend/.output/public /app/public

ENV PROMPTGATE_STATIC_ASSETS_DIR=/app/public

USER 10001:10001

EXPOSE 8080 8081

CMD ["/app/promptgate", "api"]
