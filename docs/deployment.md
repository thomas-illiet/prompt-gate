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
/app/promptgate worker
```

Recommended production layout:

| Process | Replica guidance | Notes |
| --- | --- | --- |
| `migrate` | One-off job | Run before API, proxy, worker, or scheduler for each release. |
| `api` | One or more replicas | Serves browser/API traffic and optionally static frontend assets. |
| `proxy` | One or more replicas | Serves LLM traffic, subscribes to Redis reload events, and enqueues usage events. |
| `worker` | One or more replicas | Consumes Redis usage events and updates raw usage plus dashboard KPI tables. |
| `schedule` | One replica | The current scheduler has no distributed lock. |

## Runtime Dependencies

Prompt Gate requires:

- PostgreSQL for durable data
- Redis for sessions, auth and OPA decision caches, group/provider/subscription
  snapshots, rolling quota state and reset barriers, config reload events, and
  usage event streams
- an independently operated OPA server for every proxy decision
- Keycloak or another OIDC-compatible provider
- public API, proxy, and frontend origins configured with `PROMPTGATE_*`

See [Environment variables](environment.md) for the full configuration
reference.

## Deployment Order

1. Provision PostgreSQL, Redis, and OPA.
2. Configure the OIDC client and callback URL:
   `https://api.example.com/auth/callback`.
3. Create runtime secrets:
   `PROMPTGATE_JWT_SECRET`, `PROMPTGATE_SECRETS_KEY`, database credentials,
   Redis credentials, OIDC client secret when needed, and
   `PROMPTGATE_ADMIN_API_KEY` when enabling non-browser administration. API and
   proxy reuse the same `PROMPTGATE_SECRETS_KEY` value for Model Lab tickets.
4. Run migrations with `/app/promptgate migrate`.
5. Start the API with `/app/promptgate api`.
6. Start the proxy with `/app/promptgate proxy`.
7. Start one or more workers with `/app/promptgate worker`.
8. Start the scheduler with `/app/promptgate schedule`.
9. Configure providers, users, service accounts, tokens, and the external OPA policy
   through the admin API or frontend.

Model Lab is always enabled for `user` and `admin` roles. Configure the same
browser-visible HTTP(S) URL in `PROMPTGATE_PROXY_BASE_URL` on API and proxy. If
that URL contains a routing prefix such as the
Helm chart's `/remote`, the edge must strip it before forwarding to the proxy.
Control exposure through roles and access groups. Verify that tickets are one-time,
three-stream limits hold, quota/accounting are exact, and a unique canary prompt
is absent from logs, Redis event payloads, PostgreSQL content, OTel spans,
browser storage, URLs, and error responses before broadening access. Rolling
back the release invalidates no durable Model Lab state; outstanding tickets
expire after their configured short TTL.

When upgrading from the one-turn playground to multi-turn chat, deploy the API
and proxy before the frontend. The expanded validator remains compatible with
old one-message requests, while the new frontend requires the backend to accept
alternating conversation history.

The user pre-creation migration makes the OIDC subject and last-login columns
nullable. During an upgrade to the first version that supports pre-creation,
run the migration, replace every API replica, and only then deploy or use the
frontend action and `POST /api/v1/admin/users`. Do not create an active limited,
SSO-unlinked user while older API replicas remain in service, because those binaries
still expect the two columns to be non-null when reading administration responses.

The scheduler runs monthly reset-credit distribution and pending-reset
reconciliation immediately on startup. Always complete migrations before
starting the scheduler, and keep it at one replica unless the deployment adds
external job locking.

## Container Examples

Run migrations:

```sh
docker run --rm \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate migrate
```

Run the API:

If administration-key access is enabled, populate
`PROMPTGATE_ADMIN_API_KEY` from a secret manager before running this command.
Passing `-e PROMPTGATE_ADMIN_API_KEY` forwards the environment value without
embedding the secret in the command; omit that line to keep the feature
disabled.

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
  -e PROMPTGATE_ADMIN_API_KEY \
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

## Administration API Key

Inject `PROMPTGATE_ADMIN_API_KEY` only into API containers. The proxy, worker,
scheduler, and migration processes do not use it. The Docker Compose example
does this under `services.api.environment` and leaves the feature disabled when
the interpolated host variable is unset or empty. OIDC configuration remains
required even when administration-key access is enabled.

The credential is global and grants the full `/api/v1/admin/**` surface,
including destructive operations, without per-operator attribution. Store it
in a secret manager, expose the API over HTTPS, and use the header only from
trusted CLI or server workloads. Do not inject it into frontend builds.

The API reads the value at startup. To rotate it, update the backing secret and
restart every API replica or trigger a Deployment rollout. There is no period
where one API process accepts both the old and new keys, so coordinate the
client cutover with the restart.

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

Run the worker:

```sh
docker run --rm \
  -e PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@db:5432/promptgate?sslmode=disable \
  -e PROMPTGATE_REDIS_URL=redis://redis:6379/0 \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate worker
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
- Prefer `PROMPTGATE_PROXY_TRUSTED_PROXIES` with explicit ingress or
  reverse-proxy CIDRs when the proxy needs the real client IP.
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

To preserve the real client IP with ingress-nginx, make sure the controller
actually receives and forwards it before enabling Prompt Gate trust. Depending
on your load balancer, this can require `externalTrafficPolicy: Local` on the
ingress controller Service, `use-forwarded-headers`, `enable-real-ip`, and a
correct `proxy-real-ip-cidr` in the ingress-nginx ConfigMap. Use the real CIDRs
for your load balancer, ingress nodes, or upstream proxies. See the
[ingress-nginx ConfigMap reference](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/),
[source IP notes](https://kubernetes.github.io/ingress-nginx/user-guide/miscellaneous/#source-ip-address),
and [externalTrafficPolicy FAQ](https://kubernetes.github.io/ingress-nginx/faq/#client-ipaddress-on-single-node-cluster).

For HAProxy Ingress, ensure the controller overwrites or sanitizes inbound
`X-Forwarded-For` before forwarding the client address. Temporarily enable
`PROMPTGATE_PROXY_CLIENT_IP_DEBUG=true`, send a known external request, and use
the structured `peer_ip` field to determine whether Prompt Gate sees an HAProxy
pod IP, node IP, or SNAT address. Configure only the narrow stable CIDR that
contains that peer in `PROMPTGATE_PROXY_TRUSTED_PROXIES`, keep
`PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS=false`, then disable the diagnostic.
If the trusted CIDR is shared with other workloads, restrict the proxy Service
with a NetworkPolicy selecting the HAProxy namespace and controller pods.

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
  intentionally rotating encrypted provider secrets.
- Run exactly one scheduler replica unless you add external job locking.
- Ensure at least one supported enabled provider exists before starting proxy
  replicas.

Related docs:

- [Architecture](architecture.md)
- [Proxy runtime](proxy.md)
- [Scheduler](scheduler.md)
- [Security model](security.md)
- [Release process](release.md)
