# Deployment

Prompt Gate Backend is deployed as a Docker image published to GitHub Container
Registry:

```text
ghcr.io/thomas-illiet/prompt-gate
```

The same image contains the backend binary and the generated Nuxt static
frontend assets.

## Process Layout

The image exposes:

- `8080` for the API server
- `8081` for the LLM proxy server

The default command starts the API:

```text
/app/promptgate api
```

Use explicit commands for the other processes:

```text
/app/promptgate proxy
/app/promptgate migrate
/app/promptgate schedule
```

Recommended production layout:

| Process | Replica guidance | Notes |
| --- | --- | --- |
| `migrate` | One-off job | Run before API, proxy, or scheduler for each release. |
| `api` | One or more replicas | Serves browser/API traffic and optionally static frontend assets. |
| `proxy` | One or more replicas | Serves LLM traffic and subscribes to Redis reload events. |
| `schedule` | One replica | The current scheduler has no distributed lock. |

## Runtime Dependencies

Prompt Gate requires:

- PostgreSQL for durable data
- Redis for sessions, auth cache, snapshots, and config reload events
- Keycloak or another OIDC-compatible provider
- public API, proxy, and frontend origins configured with `PROMPTGATE_*`

See [Environment variables](environment.md) for the full configuration
reference.

## Deployment Order

1. Provision PostgreSQL and Redis.
2. Configure the OIDC client and callback URL:
   `https://api.example.com/auth/callback`.
3. Create runtime secrets:
   `PROMPTGATE_JWT_SECRET`, `PROMPTGATE_SECRETS_KEY`, database credentials,
   Redis credentials, and OIDC client secret when needed.
4. Run migrations with `/app/promptgate migrate`.
5. Start the API with `/app/promptgate api`.
6. Start the proxy with `/app/promptgate proxy`.
7. Start the scheduler with `/app/promptgate schedule`.
8. Configure provider, MCP, firewall, users, service accounts, and tokens
   through the admin API or frontend.

## Container Examples

Run migrations:

```sh
docker run --rm \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate migrate
```

Run the API:

```sh
docker run --rm \
  -p 8080:8080 \
  -v /path/to/keycloak-ca.pem:/run/secrets/keycloak-ca.pem:ro \
  -e PROMPTGATE_PORT=8080 \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  -e PROMPTGATE_REDIS_URL=redis://redis:6379/0 \
  -e PROMPTGATE_KEYCLOAK_ISSUER_URL=https://keycloak.example.com/realms/promptgate \
  -e PROMPTGATE_KEYCLOAK_JWKS_URL=https://keycloak.example.com/realms/promptgate/protocol/openid-connect/certs \
  -e PROMPTGATE_KEYCLOAK_CLIENT_ID=promptgate-backend \
  -e PROMPTGATE_KEYCLOAK_CLIENT_SECRET=change-me \
  -e PROMPTGATE_KEYCLOAK_CA_CERT_PATH=/run/secrets/keycloak-ca.pem \
  -e PROMPTGATE_FRONTEND_BASE_URL=https://app.example.com \
  -e PROMPTGATE_BACKEND_BASE_URL=https://api.example.com \
  -e PROMPTGATE_PROXY_BASE_URL=https://proxy.example.com \
  -e PROMPTGATE_JWT_SECRET=change-me-change-me-change-me-32 \
  -e PROMPTGATE_SECRETS_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY= \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0
```

Omit the CA certificate volume and `PROMPTGATE_KEYCLOAK_CA_CERT_PATH` when
Keycloak uses a publicly trusted certificate.

Run the proxy:

```sh
docker run --rm \
  -p 8081:8081 \
  -e PROMPTGATE_PROXY_PORT=8081 \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  -e PROMPTGATE_REDIS_URL=redis://redis:6379/0 \
  -e PROMPTGATE_FRONTEND_BASE_URL=https://app.example.com \
  -e PROMPTGATE_JWT_SECRET=change-me-change-me-change-me-32 \
  -e PROMPTGATE_SECRETS_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY= \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate proxy
```

Run the scheduler:

```sh
docker run --rm \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  -e PROMPTGATE_REDIS_URL=redis://redis:6379/0 \
  -e PROMPTGATE_JWT_SECRET=change-me-change-me-change-me-32 \
  -e PROMPTGATE_SECRETS_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY= \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate schedule
```

## Static Frontend Assets

The Dockerfile builds the Nuxt frontend and copies generated assets to:

```text
/app/public
```

The runtime image sets:

```sh
PROMPTGATE_STATIC_ASSETS_DIR=/app/public
```

When set, the API process serves frontend files from `/` and falls back to the
SPA shell for frontend routes.

## Reverse Proxy Notes

- Route browser/API traffic to the API process.
- Route LLM SDK traffic to the proxy process.
- Set `PROMPTGATE_BACKEND_BASE_URL` to the browser-visible API origin.
- Set `PROMPTGATE_FRONTEND_BASE_URL` to the browser-visible frontend origin.
- Set `PROMPTGATE_PROXY_BASE_URL` when the proxy is exposed on a different
  origin, path, or externally mapped port.
- Enable `PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS` only when the proxy is behind
  trusted infrastructure that sanitizes forwarded headers.

## Health Checks

API:

```text
GET /health
```

Returns `200 OK` when the API can reach the database and `503 Service
Unavailable` when the database check fails.

Proxy:

```text
GET /health
```

Returns `200 OK` with `{"status":"ok"}` when the proxy process is serving.

## Production Notes

- Use immutable version tags such as `v0.1.0` for production deployments.
- Avoid deploying `latest` to production unless the platform also records the
  resolved image digest.
- Store all secrets in a secret manager.
- Keep `PROMPTGATE_SECRETS_KEY` stable across deployments unless you are
  intentionally rotating encrypted provider and MCP secrets.
- Run exactly one scheduler replica unless you add external job locking.
- Ensure at least one supported enabled provider exists before starting proxy
  replicas.

Related docs:

- [Architecture](architecture.md)
- [Proxy runtime](proxy.md)
- [Scheduler](scheduler.md)
- [Security model](security.md)
- [Release process](release.md)
