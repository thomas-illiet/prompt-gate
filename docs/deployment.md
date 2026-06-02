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
  -v /path/to/custom-ca.pem:/run/secrets/custom-ca.pem:ro \
  -e PROMPTGATE_PORT=8080 \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  -e PROMPTGATE_REDIS_URL=redis://redis:6379/0 \
  -e PROMPTGATE_KEYCLOAK_ISSUER_URL=https://keycloak.example.com/realms/promptgate \
  -e PROMPTGATE_KEYCLOAK_JWKS_URL=https://keycloak.example.com/realms/promptgate/protocol/openid-connect/certs \
  -e PROMPTGATE_KEYCLOAK_CLIENT_ID=promptgate-backend \
  -e PROMPTGATE_KEYCLOAK_CLIENT_SECRET=change-me \
  -e PROMPTGATE_CA_FILE=/run/secrets/custom-ca.pem \
  -e PROMPTGATE_FRONTEND_BASE_URL=https://app.example.com \
  -e PROMPTGATE_BACKEND_BASE_URL=https://api.example.com \
  -e PROMPTGATE_PROXY_BASE_URL=https://proxy.example.com \
  -e PROMPTGATE_JWT_SECRET=change-me-change-me-change-me-32 \
  -e PROMPTGATE_SECRETS_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY= \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0
```

Omit the CA certificate volume and `PROMPTGATE_CA_FILE` when Keycloak and
monitored HTTPS services use publicly trusted certificates.

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

## Kubernetes NGINX Ingress

When the API image serves the generated static frontend, expose it at the host
root without a rewrite:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-api
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - promptgate.example.com
      secretName: promptgate-tls
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: promptgate-api
                port:
                  number: 8080
```

Use matching public URLs:

```sh
PROMPTGATE_FRONTEND_BASE_URL=https://promptgate.example.com
PROMPTGATE_BACKEND_BASE_URL=https://promptgate.example.com
PROMPTGATE_CORS_ALLOWED_ORIGINS=https://promptgate.example.com
```

Register the OIDC redirect URI as:

```text
https://promptgate.example.com/auth/callback
```

For a separate frontend service behind the same host, route `/auth/*` and
`/api/v1/*` to the API service, `/bridge/*` to the proxy service with the
`/bridge` prefix stripped, and `/` to the frontend service. Keep the rules that
need rewrites in separate Ingress objects because nginx applies
`rewrite-target` to every path in the same object.

If `PROMPTGATE_BACKEND_BASE_URL` includes `/api`, add a compatibility rule for
the OIDC callback:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-api-auth-compat
  annotations:
    nginx.ingress.kubernetes.io/use-regex: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /auth/$2
spec:
  ingressClassName: nginx
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /api/auth(/|$)(.*)
            pathType: ImplementationSpecific
            backend:
              service:
                name: promptgate-api
                port:
                  number: 8080
```

For the LLM proxy route:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-proxy
  annotations:
    nginx.ingress.kubernetes.io/use-regex: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /$2
spec:
  ingressClassName: nginx
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /bridge(/|$)(.*)
            pathType: ImplementationSpecific
            backend:
              service:
                name: promptgate-proxy
                port:
                  number: 8081
```

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
